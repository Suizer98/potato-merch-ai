#!/usr/bin/env python3
import json
import os
import sys
import time
import urllib.error
import urllib.request

CRM_URL = os.environ.get("CRM_URL", "http://server:3000")
ORIGIN = os.environ.get("SERVER_URL", "http://localhost:3000")
ADMIN_EMAIL = os.environ.get("ADMIN_EMAIL", "admin@example.com")
ADMIN_PASSWORD = os.environ.get("ADMIN_PASSWORD", "admin123")
ADMIN_FIRST_NAME = os.environ.get("ADMIN_FIRST_NAME", "Admin")
ADMIN_LAST_NAME = os.environ.get("ADMIN_LAST_NAME", "Owner")
ADMIN_JOB_TITLE = os.environ.get("ADMIN_JOB_TITLE", "Owner")
WORKSPACE_NAME = os.environ.get("WORKSPACE_NAME", "Potato Merch")
STATE_FILE = "/state/seeded.flag"

META = "/metadata"
DATA = "/graphql"


def log(msg):
    print(f"[seed] {msg}", flush=True)


def post(path, payload, token=None):
    headers = {"Content-Type": "application/json", "Origin": ORIGIN}
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(
        CRM_URL + path, data=json.dumps(payload).encode(), headers=headers, method="POST"
    )
    try:
        with urllib.request.urlopen(req, timeout=90) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as exc:
        body = exc.read().decode()
        try:
            return json.loads(body)
        except Exception:
            return {"errors": [{"message": f"HTTP {exc.code}: {body[:200]}"}]}
    except Exception as exc:
        return {"errors": [{"message": str(exc)}]}


def gql(path, query, variables=None, token=None):
    payload = {"query": query}
    if variables is not None:
        payload["variables"] = variables
    return post(path, payload, token=token)


def gql_errors(resp):
    return resp.get("errors") or []


def wait_healthy():
    log(f"Waiting for CRM at {CRM_URL} ...")
    for _ in range(180):
        try:
            with urllib.request.urlopen(CRM_URL + "/healthz", timeout=5) as r:
                if r.status == 200:
                    log("CRM is healthy.")
                    return True
        except Exception:
            pass
        time.sleep(3)
    log("ERROR: CRM did not become healthy in time.")
    return False


def provision_new_workspace():
    log("Step 1: signUp admin (creates workspace-agnostic user)")
    r = gql(
        META,
        "mutation($e:String!,$p:String!){ signUp(email:$e,password:$p){ "
        "tokens { accessOrWorkspaceAgnosticToken { token } } } }",
        {"e": ADMIN_EMAIL, "p": ADMIN_PASSWORD},
    )
    if gql_errors(r):
        log(f"  signUp failed: {json.dumps(r)[:300]}")
        return None
    agnostic = r["data"]["signUp"]["tokens"]["accessOrWorkspaceAgnosticToken"]["token"]

    log(f"Step 2: signUpInNewWorkspace '{WORKSPACE_NAME}'")
    r = gql(
        META,
        "mutation($i:SignUpInNewWorkspaceInput){ signUpInNewWorkspace(input:$i){ "
        "loginToken { token } workspace { id } } }",
        {"i": {"displayName": WORKSPACE_NAME}},
        token=agnostic,
    )
    if gql_errors(r):
        log(f"  signUpInNewWorkspace failed: {json.dumps(r)[:300]}")
        return None
    login_token = r["data"]["signUpInNewWorkspace"]["loginToken"]["token"]

    log("Step 3: exchange loginToken for workspace access token")
    r = gql(
        META,
        "mutation($t:String!,$o:String!){ getAuthTokensFromLoginToken(loginToken:$t,origin:$o){ "
        "tokens { accessOrWorkspaceAgnosticToken { token } } } }",
        {"t": login_token, "o": ORIGIN},
    )
    if gql_errors(r):
        log(f"  token exchange failed: {json.dumps(r)[:300]}")
        return None
    access = r["data"]["getAuthTokensFromLoginToken"]["tokens"]["accessOrWorkspaceAgnosticToken"]["token"]

    log("Step 4: activateWorkspace (provisions schema + standard metadata)")
    r = gql(
        META,
        "mutation($d:ActivateWorkspaceInput!){ activateWorkspace(data:$d){ id activationStatus } }",
        {"d": {"displayName": WORKSPACE_NAME}},
        token=access,
    )
    if gql_errors(r):
        log(f"  activateWorkspace failed: {json.dumps(r)[:300]}")
        return False
    log(f"  workspace status: {r['data']['activateWorkspace']['activationStatus']}")
    return True


def get_access_token_via_signin():
    r = gql(
        META,
        "mutation($e:String!,$p:String!){ signIn(email:$e,password:$p){ "
        "availableWorkspaces { availableWorkspacesForSignIn { id loginToken } } } }",
        {"e": ADMIN_EMAIL, "p": ADMIN_PASSWORD},
    )
    if gql_errors(r):
        return None
    workspaces = r["data"]["signIn"]["availableWorkspaces"]["availableWorkspacesForSignIn"]
    if not workspaces:
        return None
    login_token = workspaces[0]["loginToken"]
    r = gql(
        META,
        "mutation($t:String!,$o:String!){ getAuthTokensFromLoginToken(loginToken:$t,origin:$o){ "
        "tokens { accessOrWorkspaceAgnosticToken { token } } } }",
        {"t": login_token, "o": ORIGIN},
    )
    if gql_errors(r):
        return None
    return r["data"]["getAuthTokensFromLoginToken"]["tokens"]["accessOrWorkspaceAgnosticToken"]["token"]


def complete_admin_onboarding(token):
    """Fill admin profile and skip remaining onboarding so UI lands in the app."""
    log(f"Completing admin profile as {ADMIN_FIRST_NAME} {ADMIN_LAST_NAME} (Owner)")
    r = gql(
        META,
        "query { currentUser { onboardingStatus isWorkspaceCreator "
        "workspaceMember { id roles { label } } } }",
        token=token,
    )
    if gql_errors(r):
        log(f"  WARN currentUser: {json.dumps(gql_errors(r))[:200]}")
        return False

    user = r["data"]["currentUser"]
    member = user["workspaceMember"]
    roles = [role["label"] for role in (member.get("roles") or [])]
    log(f"  roles={roles} creator={user.get('isWorkspaceCreator')} status={user.get('onboardingStatus')}")

    r = gql(
        META,
        "mutation($i:UpdateWorkspaceMemberSettingsInput!){ updateWorkspaceMemberSettings(input:$i) }",
        {
            "i": {
                "workspaceMemberId": member["id"],
                "update": {
                    "name": {"firstName": ADMIN_FIRST_NAME, "lastName": ADMIN_LAST_NAME},
                    "jobTitle": ADMIN_JOB_TITLE,
                },
            }
        },
        token=token,
    )
    if gql_errors(r):
        log(f"  WARN updateWorkspaceMemberSettings: {json.dumps(gql_errors(r))[:200]}")

    # Clear remaining onboarding gates (order matches Twenty's status priority).
    for label, query, variables in [
        ("skipSyncEmailOnboardingStep", "mutation { skipSyncEmailOnboardingStep { success } }", None),
        (
            "triggerInstallAppsOnboardingStep",
            "mutation($ids:[String!]!){ triggerInstallAppsOnboardingStep(universalIdentifiers:$ids){ success } }",
            {"ids": []},
        ),
        ("sendInvitations", "mutation($e:[String!]!){ sendInvitations(emails:$e){ success } }", {"e": []}),
        ("completeBookCallOnboardingStep", "mutation { completeBookCallOnboardingStep { success } }", None),
    ]:
        r = gql(META, query, variables, token=token)
        if gql_errors(r):
            log(f"  WARN {label}: {json.dumps(gql_errors(r))[:200]}")

    r = gql(META, "query { currentUser { onboardingStatus workspaceMember { name { firstName lastName } roles { label } } } }", token=token)
    if gql_errors(r):
        return False
    final = r["data"]["currentUser"]
    status = final.get("onboardingStatus")
    name = final["workspaceMember"]["name"]
    log(f"  final status={status} name={name['firstName']} {name['lastName']}")
    return status == "COMPLETED"


def list_objects(token):
    r = gql(META, "query { objects(paging:{first:500}){ edges { node { id nameSingular } } } }", token=token)
    edges = (((r.get("data") or {}).get("objects")) or {}).get("edges") or []
    return {e["node"]["nameSingular"]: e["node"]["id"] for e in edges}


def create_object(token, obj):
    r = gql(
        META,
        "mutation($i:CreateOneObjectInput!){ createOneObject(input:$i){ id nameSingular } }",
        {"i": {"object": obj}},
        token=token,
    )
    if gql_errors(r):
        log(f"  createObject {obj['nameSingular']}: {json.dumps(gql_errors(r))[:200]}")
        return None
    return r["data"]["createOneObject"]["id"]


def create_field(token, field):
    variants = [field]
    if "settings" in field:
        no_settings = dict(field)
        no_settings.pop("settings")
        variants.append(no_settings)
    for variant in variants:
        r = gql(
            META,
            "mutation($i:CreateOneFieldMetadataInput!){ createOneField(input:$i){ id name } }",
            {"i": {"field": variant}},
            token=token,
        )
        if not gql_errors(r):
            return True
        msg = json.dumps(gql_errors(r))
        if "already exists" in msg or "duplicate" in msg.lower():
            return True
    log(f"  field {field['name']}: {msg[:180]}")
    return False


def schema_not_ready(errors):
    blob = json.dumps(errors)
    return "Unknown type" in blob or "Cannot query field" in blob


def create_record(token, cap, data, ready_timeout=0):
    query = "mutation($v:%sCreateInput!){ create%s(data:$v){ id } }" % (cap, cap)
    deadline = time.time() + ready_timeout
    while True:
        r = gql(DATA, query, {"v": data}, token=token)
        errs = gql_errors(r)
        if not errs:
            return True, None
        if schema_not_ready(errs) and time.time() < deadline:
            time.sleep(4)
            continue
        return False, json.dumps(errs)[:180]


PRODUCTS_FIELDS = [
    {"name": "sku", "label": "SKU", "type": "TEXT"},
    {"name": "description", "label": "Description", "type": "TEXT"},
    {"name": "price", "label": "Price", "type": "NUMBER", "settings": {"dataType": "float", "decimals": 2}},
    # Original ticket price; price holds the current (discounted) price.
    {"name": "compareAtPrice", "label": "Compare At Price", "type": "NUMBER", "settings": {"dataType": "float", "decimals": 2}},
    {"name": "stock", "label": "Stock", "type": "NUMBER", "settings": {"dataType": "int"}},
    {
        "name": "category",
        "label": "Category",
        "type": "SELECT",
        "options": [
            {"label": "T-Shirts", "value": "TEES", "color": "green", "position": 0},
        ],
    },
    {
        "name": "season",
        "label": "Season",
        "type": "SELECT",
        "options": [
            {"label": "Season 1", "value": "SEASON_1", "color": "gray", "position": 0},
            {"label": "Season 2", "value": "SEASON_2", "color": "blue", "position": 1},
            {"label": "Season 3", "value": "SEASON_3", "color": "green", "position": 2},
        ],
    },
    {
        "name": "availability",
        "label": "Availability",
        "type": "SELECT",
        "options": [
            {"label": "In Stock", "value": "IN_STOCK", "color": "green", "position": 0},
            {"label": "Preorder", "value": "PREORDER", "color": "yellow", "position": 1},
            {"label": "Sold Out", "value": "SOLD_OUT", "color": "red", "position": 2},
        ],
    },
    {"name": "sizes", "label": "Sizes", "type": "TEXT"},
    {"name": "imageUrl", "label": "Image URL", "type": "TEXT"},
    {"name": "isOnSale", "label": "On Sale", "type": "BOOLEAN"},
]

TICKETS_FIELDS = [
    {"name": "customerEmail", "label": "Customer Email", "type": "TEXT"},
    {"name": "customerName", "label": "Customer Name", "type": "TEXT"},
    {
        "name": "category",
        "label": "Category",
        "type": "SELECT",
        "options": [
            {"label": "Technical", "value": "TECHNICAL", "color": "blue", "position": 0},
            {"label": "Billing", "value": "BILLING", "color": "green", "position": 1},
            {"label": "Product", "value": "PRODUCT", "color": "orange", "position": 2},
            {"label": "Account", "value": "ACCOUNT", "color": "purple", "position": 3},
        ],
    },
    {
        "name": "priority",
        "label": "Priority",
        "type": "SELECT",
        "options": [
            {"label": "Low", "value": "LOW", "color": "gray", "position": 0},
            {"label": "Medium", "value": "MEDIUM", "color": "yellow", "position": 1},
            {"label": "High", "value": "HIGH", "color": "red", "position": 2},
        ],
    },
    {
        "name": "status",
        "label": "Status",
        "type": "SELECT",
        "options": [
            {"label": "New", "value": "NEW", "color": "blue", "position": 0},
            {"label": "Triage", "value": "TRIAGE", "color": "purple", "position": 1},
            {"label": "In Progress", "value": "IN_PROGRESS", "color": "yellow", "position": 2},
            {"label": "Escalated", "value": "ESCALATED", "color": "orange", "position": 3},
            {"label": "Resolved", "value": "RESOLVED", "color": "green", "position": 4},
            {"label": "Closed", "value": "CLOSED", "color": "gray", "position": 5},
        ],
    },
    {"name": "description", "label": "Description", "type": "TEXT"},
    {"name": "assignedAgent", "label": "Assigned Agent", "type": "TEXT"},
    {"name": "resolution", "label": "Resolution", "type": "TEXT"},
]

CUSTOMERS_FIELDS = [
    {"name": "email", "label": "Email", "type": "TEXT"},
    {"name": "firstName", "label": "First Name", "type": "TEXT"},
    {"name": "lastName", "label": "Last Name", "type": "TEXT"},
    {"name": "phone", "label": "Phone", "type": "TEXT"},
    {"name": "mailingAddress", "label": "Address", "type": "TEXT"},
]

ORDERS_FIELDS = [
    {"name": "orderNumber", "label": "Order Number", "type": "TEXT"},
    {"name": "total", "label": "Total", "type": "NUMBER", "settings": {"dataType": "float", "decimals": 2}},
    {
        "name": "status",
        "label": "Status",
        "type": "SELECT",
        "options": [
            {"label": "Pending", "value": "PENDING", "color": "yellow", "position": 0},
            {"label": "Paid", "value": "PAID", "color": "blue", "position": 1},
            {"label": "Shipped", "value": "SHIPPED", "color": "purple", "position": 2},
            {"label": "Delivered", "value": "DELIVERED", "color": "green", "position": 3},
            {"label": "Cancelled", "value": "CANCELLED", "color": "red", "position": 4},
        ],
    },
    {"name": "customerEmail", "label": "Customer Email", "type": "TEXT"},
]

IMG = "https://coresg-normal.trae.ai/api/ide/v1/text_to_image?prompt="
TEE_SIZES = "S,M,L,XL,XXL"
FIT_SIZES = "XS,S,M,L,XL"

PRODUCTS = [
    {
        "name": "Couch Potato Tee", "sku": "TEE-COUCH-001", "category": "TEES", "season": "SEASON_1",
        "price": 25.20, "compareAtPrice": 42.00, "isOnSale": True, "availability": "IN_STOCK", "stock": 180,
        "sizes": TEE_SIZES,
        "description": "Cartoon potato slumped on a couch with a TV remote and a bag of chips, printed big across the chest. 100% heavyweight cotton (6 oz). Machine wash cold inside out; tumble dry low. Model is 6'2\" and wearing a Large.",
        "imageUrl": IMG + "cartoon%20potato%20character%20lounging%20on%20couch%20with%20tv%20remote%20tshirt%20print",
    },
    {
        "name": "Gym Spud Tee", "sku": "TEE-GYM-002", "category": "TEES", "season": "SEASON_2",
        "price": 25.20, "compareAtPrice": 42.00, "isOnSale": True, "availability": "IN_STOCK", "stock": 140,
        "sizes": TEE_SIZES,
        "description": "Buff cartoon potato curling dumbbells in a sweatband, with 'NEVER SKIP TATER DAY' arched underneath. 100% combed cotton (6 oz).",
        "imageUrl": IMG + "cartoon%20muscular%20potato%20character%20lifting%20dumbbells%20with%20sweatband%20tshirt%20print",
    },
    {
        "name": "Angry Fry Tee", "sku": "TEE-FRY-003", "category": "TEES", "season": "SEASON_3",
        "price": 26.40, "compareAtPrice": 44.00, "isOnSale": True, "availability": "IN_STOCK", "stock": 160,
        "sizes": TEE_SIZES,
        "description": "Furious cartoon potato hurling french fries, veins popping, comic halftone shading. Screenprinted front graphic with a small fry doodle on the sleeve.",
        "imageUrl": IMG + "angry%20cartoon%20potato%20character%20throwing%20french%20fries%20comic%20style%20tshirt%20print",
    },
    {
        "name": "Sleepy Tater Tee", "sku": "TEE-SLEEPY-004", "category": "TEES", "season": "SEASON_2",
        "price": 24.00, "compareAtPrice": 40.00, "isOnSale": True, "availability": "IN_STOCK", "stock": 120,
        "sizes": TEE_SIZES,
        "description": "Snoozing cartoon potato in a nightcap with little Zzz's floating off the shoulder. Soft-hand print, relaxed fit with dropped shoulders.",
        "imageUrl": IMG + "sleepy%20cartoon%20potato%20character%20in%20nightcap%20with%20zzz%20tshirt%20print",
    },
    {
        "name": "DJ Mash Tee", "sku": "TEE-DJ-005", "category": "TEES", "season": "SEASON_3",
        "price": 26.40, "compareAtPrice": 44.00, "isOnSale": True, "availability": "PREORDER", "stock": 0,
        "sizes": TEE_SIZES,
        "description": "Cartoon potato DJ behind the decks in oversized headphones, speaker cones blasting mash. Preorder: ships in 3-4 weeks.",
        "imageUrl": IMG + "cartoon%20potato%20character%20dj%20with%20headphones%20behind%20turntables%20tshirt%20print",
    },
    {
        "name": "Skater Spud Tee", "sku": "TEE-SKATE-006", "category": "TEES", "season": "SEASON_1",
        "price": 25.20, "compareAtPrice": 42.00, "isOnSale": True, "availability": "SOLD_OUT", "stock": 0,
        "sizes": TEE_SIZES,
        "description": "Cartoon potato mid-kickflip in a backwards cap, board spinning under a scribbled skyline. Season 1 archive piece.",
        "imageUrl": IMG + "cartoon%20potato%20character%20skateboarding%20kickflip%20with%20cap%20tshirt%20print",
    },
    {
        "name": "Chef Potato Tee", "sku": "TEE-CHEF-007", "category": "TEES", "season": "SEASON_2",
        "price": 25.20, "compareAtPrice": 42.00, "isOnSale": True, "availability": "IN_STOCK", "stock": 95,
        "sizes": TEE_SIZES,
        "description": "Cartoon potato in a chef hat tossing hash browns in a frying pan, with a tiny 'MASHED TO ORDER' banner. Front screenprint on cream cotton.",
        "imageUrl": IMG + "cartoon%20potato%20character%20chef%20hat%20tossing%20food%20in%20frying%20pan%20tshirt%20print",
    },
    {
        "name": "Astro Potato Tee", "sku": "TEE-ASTRO-008", "category": "TEES", "season": "SEASON_3",
        "price": 27.60, "compareAtPrice": 46.00, "isOnSale": True, "availability": "IN_STOCK", "stock": 200,
        "sizes": TEE_SIZES,
        "description": "Cartoon potato astronaut drifting through space with french fries as shooting stars. Glow-adjacent pastel palette on midnight black.",
        "imageUrl": IMG + "cartoon%20potato%20astronaut%20floating%20in%20space%20with%20french%20fry%20stars%20tshirt%20print",
    },
    {
        "name": "Ninja Tater Tee", "sku": "TEE-NINJA-009", "category": "TEES", "season": "SEASON_3",
        "price": 25.20, "compareAtPrice": 42.00, "isOnSale": True, "availability": "SOLD_OUT", "stock": 0,
        "sizes": TEE_SIZES,
        "description": "Masked cartoon potato ninja mid-leap with a katana and a trail of potato-peel smoke. Two-colour print on faded black.",
        "imageUrl": IMG + "cartoon%20potato%20ninja%20character%20with%20katana%20jumping%20tshirt%20print",
    },
    {
        "name": "Baby Potato Tee", "sku": "TEE-BABY-010", "category": "TEES", "season": "SEASON_2",
        "price": 25.20, "compareAtPrice": 42.00, "isOnSale": True, "availability": "IN_STOCK", "stock": 110,
        "sizes": FIT_SIZES,
        "description": "Chubby cartoon baby potato with rosy cheeks and tiny sprout arms, puff-printed on a fitted baby tee. Ribbed collar, cropped length.",
        "imageUrl": IMG + "cute%20chubby%20cartoon%20baby%20potato%20character%20with%20rosy%20cheeks%20tshirt%20print",
    },
]

TICKETS = [
    {"name": "Couch Potato Tee never arrived", "customerEmail": "john.smith@gmail.com", "customerName": "John Smith", "category": "BILLING", "priority": "HIGH", "status": "NEW", "description": "Order ORD-1042 placed Aug 1 for the Couch Potato Tee, charged 25.20 USD, still no tracking number. Please refund or ship immediately.", "assignedAgent": "Priya Raman"},
    {"name": "Checkout fails with Apple Pay", "customerEmail": "sarah.wong@outlook.com", "customerName": "Sarah Wong", "category": "TECHNICAL", "priority": "MEDIUM", "status": "IN_PROGRESS", "description": "iPhone Safari, Apple Pay confirms then checkout returns payment method declined. Trying to buy the Gym Spud Tee. Card works on other stores.", "assignedAgent": "Marco Diaz"},
    {"name": "Will the Skater Spud Tee restock?", "customerEmail": "mike.torres@proton.me", "customerName": "Mike Torres", "category": "PRODUCT", "priority": "LOW", "status": "TRIAGE", "description": "Missed the Season 1 drop. Is the Skater Spud Tee restocking in size M, and is there a waitlist?", "assignedAgent": "Priya Raman"},
    {"name": "Wrong size shipped - ordered L got M", "customerEmail": "lisa.ng@yahoo.com", "customerName": "Lisa Ng", "category": "ACCOUNT", "priority": "HIGH", "status": "ESCALATED", "description": "Received ORD-1066 Couch Potato Tee labelled M but I ordered L. Unworn with tags - requesting exchange.", "assignedAgent": "Marco Diaz"},
    {"name": "DJ Mash Tee preorder ship date?", "customerEmail": "david.kim@aol.com", "customerName": "David Kim", "category": "PRODUCT", "priority": "MEDIUM", "status": "NEW", "description": "Preordered the DJ Mash Tee on ORD-1073. Listing says 3-4 weeks - can you confirm the ship window before I travel?", "assignedAgent": "Priya Raman"},
    {"name": "Does the cartoon print crack after washing?", "customerEmail": "amara.osei@gmail.com", "customerName": "Amara Osei", "category": "PRODUCT", "priority": "LOW", "status": "RESOLVED", "description": "Asked whether the Sleepy Tater cartoon print cracks in the wash.", "assignedAgent": "Priya Raman", "resolution": "Advised machine wash cold inside out and tumble dry low; the soft-hand print stays intact for the life of the shirt when washed that way."},
]

CUSTOMERS = [
    {"name": "John Smith", "email": "john.smith@gmail.com", "firstName": "John", "lastName": "Smith", "phone": "+1 415 555 0142", "mailingAddress": "1180 Folsom St, San Francisco, CA 94103, USA"},
    {"name": "Sarah Wong", "email": "sarah.wong@outlook.com", "firstName": "Sarah", "lastName": "Wong", "phone": "+65 8123 4455", "mailingAddress": "18 Robinson Rd, #05-12, Singapore 048547"},
    {"name": "Mike Torres", "email": "mike.torres@proton.me", "firstName": "Mike", "lastName": "Torres", "phone": "+1 512 555 0198", "mailingAddress": "902 E 6th St, Austin, TX 78702, USA"},
    {"name": "Lisa Ng", "email": "lisa.ng@yahoo.com", "firstName": "Lisa", "lastName": "Ng", "phone": "+60 12 345 6789", "mailingAddress": "12 Jalan Bukit Bintang, 55100 Kuala Lumpur, Malaysia"},
    {"name": "David Kim", "email": "david.kim@aol.com", "firstName": "David", "lastName": "Kim", "phone": "+1 213 555 0177", "mailingAddress": "3400 W 6th St, Los Angeles, CA 90020, USA"},
    {"name": "Amara Osei", "email": "amara.osei@gmail.com", "firstName": "Amara", "lastName": "Osei", "phone": "+44 20 7946 0321", "mailingAddress": "42 Rivington St, London EC2A 3BN, UK"},
    {"name": "Yuki Tanaka", "email": "yuki.tanaka@gmail.com", "firstName": "Yuki", "lastName": "Tanaka", "phone": "+81 3 6811 2233", "mailingAddress": "2-11-3 Shibuya, Shibuya City, Tokyo 150-0002, Japan"},
    {"name": "Carlos Mendes", "email": "carlos.mendes@gmail.com", "firstName": "Carlos", "lastName": "Mendes", "phone": "+55 11 91234 5678", "mailingAddress": "Rua Augusta 1200, Consolacao, Sao Paulo 01304-001, Brazil"},
    {"name": "Priya Sharma", "email": "priya.sharma@outlook.com", "firstName": "Priya", "lastName": "Sharma", "phone": "+91 98200 11223", "mailingAddress": "14 Hill Road, Bandra West, Mumbai 400050, India"},
    {"name": "Tom Becker", "email": "tom.becker@gmx.de", "firstName": "Tom", "lastName": "Becker", "phone": "+49 30 1234 5678", "mailingAddress": "Torstrasse 66, 10119 Berlin, Germany"},
    {"name": "Chloe Dubois", "email": "chloe.dubois@orange.fr", "firstName": "Chloe", "lastName": "Dubois", "phone": "+33 1 42 86 82 00", "mailingAddress": "18 Rue de Rivoli, 75004 Paris, France"},
    {"name": "Ethan Brooks", "email": "ethan.brooks@gmail.com", "firstName": "Ethan", "lastName": "Brooks", "phone": "+1 646 555 0119", "mailingAddress": "210 W 14th St, New York, NY 10011, USA"},
    {"name": "Mei Lin Chua", "email": "meilin.chua@hotmail.com", "firstName": "Mei Lin", "lastName": "Chua", "phone": "+65 9123 7788", "mailingAddress": "50 Tiong Bahru Rd, Singapore 168733"},
    {"name": "Jaden Cole", "email": "jaden.cole@icloud.com", "firstName": "Jaden", "lastName": "Cole", "phone": "+1 305 555 0166", "mailingAddress": "1450 Ocean Dr, Miami Beach, FL 33139, USA"},
    {"name": "Aisha Rahman", "email": "aisha.rahman@gmail.com", "firstName": "Aisha", "lastName": "Rahman", "phone": "+971 4 555 0143", "mailingAddress": "Alserkal Ave, Al Quoz 1, Dubai, UAE"},
    {"name": "Liam O'Connor", "email": "liam.oconnor@eircom.net", "firstName": "Liam", "lastName": "O'Connor", "phone": "+353 1 555 0121", "mailingAddress": "12 Grafton St, Dublin D02 VK65, Ireland"},
    {"name": "Nina Petrova", "email": "nina.petrova@yandex.com", "firstName": "Nina", "lastName": "Petrova", "phone": "+48 22 555 0134", "mailingAddress": "ul. Nowy Swiat 40, 00-363 Warsaw, Poland"},
    {"name": "Kofi Mensah", "email": "kofi.mensah@gmail.com", "firstName": "Kofi", "lastName": "Mensah", "phone": "+233 30 255 0177", "mailingAddress": "18 Oxford St, Osu, Accra, Ghana"},
    {"name": "Hana Kim", "email": "hana.kim@naver.com", "firstName": "Hana", "lastName": "Kim", "phone": "+82 2 555 0188", "mailingAddress": "34 Itaewon-ro, Yongsan-gu, Seoul 04348, South Korea"},
    {"name": "Ryan Walsh", "email": "ryan.walsh@gmail.com", "firstName": "Ryan", "lastName": "Walsh", "phone": "+61 2 5550 0192", "mailingAddress": "88 Crown St, Surry Hills NSW 2010, Australia"},
]

ORDERS = [
    {"name": "ORD-1042", "orderNumber": "ORD-1042", "total": 25.20, "status": "PAID", "customerEmail": "john.smith@gmail.com"},
    {"name": "ORD-1051", "orderNumber": "ORD-1051", "total": 52.80, "status": "SHIPPED", "customerEmail": "sarah.wong@outlook.com"},
    {"name": "ORD-1058", "orderNumber": "ORD-1058", "total": 26.40, "status": "DELIVERED", "customerEmail": "mike.torres@proton.me"},
    {"name": "ORD-1066", "orderNumber": "ORD-1066", "total": 25.20, "status": "DELIVERED", "customerEmail": "lisa.ng@yahoo.com"},
    {"name": "ORD-1073", "orderNumber": "ORD-1073", "total": 26.40, "status": "PENDING", "customerEmail": "david.kim@aol.com"},
    {"name": "ORD-1080", "orderNumber": "ORD-1080", "total": 49.20, "status": "PAID", "customerEmail": "amara.osei@gmail.com"},
    {"name": "ORD-1085", "orderNumber": "ORD-1085", "total": 25.20, "status": "CANCELLED", "customerEmail": "yuki.tanaka@gmail.com"},
    {"name": "ORD-1091", "orderNumber": "ORD-1091", "total": 78.00, "status": "SHIPPED", "customerEmail": "priya.sharma@outlook.com"},
]


def main():
    if os.path.exists(STATE_FILE):
        log("Already seeded (state flag present). Exiting.")
        return 0

    if not wait_healthy():
        return 1
    time.sleep(3)

    exists_resp = gql(
        META,
        "query($e:String!){ checkUserExists(email:$e){ exists } }",
        {"e": ADMIN_EMAIL},
    )
    user_exists = (((exists_resp.get("data") or {}).get("checkUserExists")) or {}).get("exists", False)

    if not user_exists:
        if not provision_new_workspace():
            log("ERROR: could not provision workspace. Aborting.")
            return 1
    else:
        log("Admin user already exists; skipping signup.")

    # Token must be minted after activation so it includes workspaceMemberId.
    token = get_access_token_via_signin()
    if not token:
        log("ERROR: could not obtain a workspace access token. Aborting.")
        return 1

    if not complete_admin_onboarding(token):
        log("WARN: admin onboarding did not reach COMPLETED; UI may still show setup steps.")

    existing = list_objects(token)

    objects = [
        ({"nameSingular": "product", "namePlural": "products", "labelSingular": "Product", "labelPlural": "Products", "icon": "IconTag"}, PRODUCTS_FIELDS),
        ({"nameSingular": "supportTicket", "namePlural": "supportTickets", "labelSingular": "Support Ticket", "labelPlural": "Support Tickets", "icon": "IconMessageCircle"}, TICKETS_FIELDS),
        ({"nameSingular": "customer", "namePlural": "customers", "labelSingular": "Customer", "labelPlural": "Customers", "icon": "IconUser"}, CUSTOMERS_FIELDS),
        ({"nameSingular": "order", "namePlural": "orders", "labelSingular": "Order", "labelPlural": "Orders", "icon": "IconShoppingCart"}, ORDERS_FIELDS),
    ]

    for obj_def, fields in objects:
        ns = obj_def["nameSingular"]
        oid = existing.get(ns)
        if oid:
            log(f"Object '{ns}' exists; ensuring {len(fields)} fields")
        else:
            log(f"Creating object '{ns}' + {len(fields)} fields")
            oid = create_object(token, obj_def)
        if not oid:
            continue
        for f in fields:
            create_field(token, {**f, "objectMetadataId": oid})

    for cap, records in [
        ("Product", PRODUCTS),
        ("Customer", CUSTOMERS),
        ("Order", ORDERS),
        ("SupportTicket", TICKETS),
    ]:
        log(f"Seeding {cap} records")
        ok = 0
        for i, rec in enumerate(records):
            success, err = create_record(token, cap, rec, ready_timeout=180 if i == 0 else 0)
            if success:
                ok += 1
            else:
                log(f"  WARN {cap}: {err}")
        log(f"{cap} records seeded: {ok}/{len(records)}")

    try:
        with open(STATE_FILE, "w") as fh:
            fh.write("seeded\n")
    except Exception as exc:
        log(f"WARN: could not write state file: {exc}")

    log("=== DONE ===")
    log(f"  Admin email    : {ADMIN_EMAIL}")
    log(f"  Admin password : {ADMIN_PASSWORD}")
    log(f"  Admin profile  : {ADMIN_FIRST_NAME} {ADMIN_LAST_NAME} ({ADMIN_JOB_TITLE})")
    log(f"  Open           : {ORIGIN}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
