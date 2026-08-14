const CRM_URL = () => process.env.CRM_URL || 'http://localhost:3000'
const ORIGIN = () => process.env.SERVER_URL || 'http://localhost:3000'
const ADMIN_EMAIL = () => process.env.ADMIN_EMAIL || 'admin@example.com'
const ADMIN_PASSWORD = () => process.env.ADMIN_PASSWORD || 'admin123'

let cachedToken = null
let cachedAt = 0
const TOKEN_TTL_MS = 10 * 60 * 1000

async function post(path, payload, token) {
  const headers = {
    'Content-Type': 'application/json',
    Origin: ORIGIN(),
  }
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${CRM_URL()}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(payload),
  })
  const text = await res.text()
  try {
    return JSON.parse(text)
  } catch {
    return { errors: [{ message: `HTTP ${res.status}: ${text.slice(0, 200)}` }] }
  }
}

function gql(path, query, variables, token) {
  const payload = { query }
  if (variables !== undefined) payload.variables = variables
  return post(path, payload, token)
}

function selectValue(field) {
  if (field == null) return null
  if (typeof field === 'string') return field
  if (typeof field === 'object' && 'value' in field) return field.value
  return String(field)
}

async function getAccessToken() {
  const now = Date.now()
  if (cachedToken && now - cachedAt < TOKEN_TTL_MS) return cachedToken

  const signIn = await gql(
    '/metadata',
    `mutation($e:String!,$p:String!){
      signIn(email:$e,password:$p){
        availableWorkspaces { availableWorkspacesForSignIn { id loginToken } }
      }
    }`,
    { e: ADMIN_EMAIL(), p: ADMIN_PASSWORD() },
  )
  if (signIn.errors?.length) {
    throw new Error(`CRM signIn failed: ${JSON.stringify(signIn.errors)}`)
  }

  const workspaces =
    signIn.data?.signIn?.availableWorkspaces?.availableWorkspacesForSignIn || []
  if (!workspaces.length) throw new Error('CRM signIn returned no workspaces')

  const exchange = await gql(
    '/metadata',
    `mutation($t:String!,$o:String!){
      getAuthTokensFromLoginToken(loginToken:$t,origin:$o){
        tokens { accessOrWorkspaceAgnosticToken { token } }
      }
    }`,
    { t: workspaces[0].loginToken, o: ORIGIN() },
  )
  if (exchange.errors?.length) {
    throw new Error(`CRM token exchange failed: ${JSON.stringify(exchange.errors)}`)
  }

  const token =
    exchange.data?.getAuthTokensFromLoginToken?.tokens?.accessOrWorkspaceAgnosticToken
      ?.token
  if (!token) throw new Error('CRM token exchange returned no token')

  cachedToken = token
  cachedAt = now
  return token
}

function mapProduct(node) {
  const compareAt = Number(node.compareAtPrice ?? 0)
  const price = Number(node.price ?? 0)
  const availability = selectValue(node.availability) || 'IN_STOCK'
  const sizes =
    typeof node.sizes === 'string'
      ? node.sizes.split(',').map((s) => s.trim()).filter(Boolean)
      : []

  return {
    id: node.id,
    name: node.name,
    sku: node.sku || '',
    description: node.description || '',
    price,
    compareAtPrice: compareAt,
    stock: Number(node.stock ?? 0),
    category: selectValue(node.category) || 'TEES',
    season: selectValue(node.season) || 'SEASON_1',
    availability,
    sizes,
    imageUrl: node.imageUrl || '',
    isOnSale: Boolean(node.isOnSale),
    soldOut: availability === 'SOLD_OUT' || Number(node.stock ?? 0) <= 0 && availability !== 'PREORDER',
  }
}

export async function fetchProductsFromCrm() {
  const token = await getAccessToken()
  const query = `query {
    products(paging: { first: 100 }) {
      edges {
        node {
          id
          name
          sku
          description
          price
          compareAtPrice
          stock
          category
          season
          availability
          sizes
          imageUrl
          isOnSale
        }
      }
    }
  }`

  let result = await gql('/graphql', query, undefined, token)
  if (result.errors?.length) {
    const alt = `query {
      products(first: 100) {
        edges {
          node {
            id
            name
            sku
            description
            price
            compareAtPrice
            stock
            category
            season
            availability
            sizes
            imageUrl
            isOnSale
          }
        }
      }
    }`
    result = await gql('/graphql', alt, undefined, token)
  }

  if (result.errors?.length) {
    throw new Error(`CRM products query failed: ${JSON.stringify(result.errors)}`)
  }

  const edges = result.data?.products?.edges || []
  return edges.map((e) => mapProduct(e.node))
}
