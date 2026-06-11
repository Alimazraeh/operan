// API client + in-browser JWT minting for the Operan portal.
// Works in non-secure contexts (plain HTTP over LAN): uuid4 uses
// getRandomValues and HMAC-SHA256 falls back to pure JS when SubtleCrypto
// is unavailable.

export const SVC = {
  tenant: "/svc/tenant",
  orchestration: "/svc/orchestration/api/v1/orchestration",
  registry: "/svc/registry",
  templates: "/svc/templates",
  memory: "/svc/memory",
  tools: "/svc/tools",
  supervision: "/svc/supervision",
  observability: "/svc/observability",
};

export const session = {
  jwt: "", tenant: "",
  get active() { return !!this.jwt; },
};

export function uuid4() {
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80;
  const h = [...b].map(x => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0,8)}-${h.slice(8,12)}-${h.slice(12,16)}-${h.slice(16,20)}-${h.slice(20)}`;
}

function sha256Bytes(msg) {
  const K = [0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,
    0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,
    0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,
    0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,
    0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,
    0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,
    0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,
    0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2];
  let H = [0x6a09e667,0xbb67ae85,0x3c6ef372,0xa54ff53a,0x510e527f,0x9b05688c,0x1f83d9ab,0x5be0cd19];
  const l = msg.length, bitLen = l * 8;
  const padded = new Uint8Array(((l + 8) >> 6 << 6) + 64);
  padded.set(msg); padded[l] = 0x80;
  new DataView(padded.buffer).setUint32(padded.length - 4, bitLen >>> 0);
  new DataView(padded.buffer).setUint32(padded.length - 8, Math.floor(bitLen / 0x100000000));
  const w = new Int32Array(64);
  const rr = (x, n) => (x >>> n) | (x << (32 - n));
  for (let off = 0; off < padded.length; off += 64) {
    const dv = new DataView(padded.buffer, off, 64);
    for (let i = 0; i < 16; i++) w[i] = dv.getUint32(i * 4);
    for (let i = 16; i < 64; i++) {
      const s0 = rr(w[i-15],7) ^ rr(w[i-15],18) ^ (w[i-15] >>> 3);
      const s1 = rr(w[i-2],17) ^ rr(w[i-2],19) ^ (w[i-2] >>> 10);
      w[i] = (w[i-16] + s0 + w[i-7] + s1) | 0;
    }
    let [a,b,c,d,e,f,g,h] = H;
    for (let i = 0; i < 64; i++) {
      const S1 = rr(e,6) ^ rr(e,11) ^ rr(e,25);
      const ch = (e & f) ^ (~e & g);
      const t1 = (h + S1 + ch + K[i] + w[i]) | 0;
      const S0 = rr(a,2) ^ rr(a,13) ^ rr(a,22);
      const mj = (a & b) ^ (a & c) ^ (b & c);
      const t2 = (S0 + mj) | 0;
      h=g; g=f; f=e; e=(d+t1)|0; d=c; c=b; b=a; a=(t1+t2)|0;
    }
    H = [(H[0]+a)|0,(H[1]+b)|0,(H[2]+c)|0,(H[3]+d)|0,(H[4]+e)|0,(H[5]+f)|0,(H[6]+g)|0,(H[7]+h)|0];
  }
  const out = new Uint8Array(32);
  const dv = new DataView(out.buffer);
  H.forEach((x, i) => dv.setUint32(i * 4, x >>> 0));
  return out;
}

function hmacSha256(keyBytes, msgBytes) {
  let key = keyBytes.length > 64 ? sha256Bytes(keyBytes) : keyBytes;
  const ipad = new Uint8Array(64).fill(0x36), opad = new Uint8Array(64).fill(0x5c);
  for (let i = 0; i < key.length; i++) { ipad[i] ^= key[i]; opad[i] ^= key[i]; }
  const inner = new Uint8Array(64 + msgBytes.length);
  inner.set(ipad); inner.set(msgBytes, 64);
  const ih = sha256Bytes(inner);
  const outer = new Uint8Array(96);
  outer.set(opad); outer.set(ih, 64);
  return sha256Bytes(outer);
}

const b64url = buf => btoa(String.fromCharCode(...new Uint8Array(buf)))
  .replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
const b64urlStr = s => b64url(new TextEncoder().encode(s));

export async function mintJWT(secret) {
  const header = b64urlStr(JSON.stringify({alg: "HS256", typ: "JWT"}));
  const payload = b64urlStr(JSON.stringify({
    sub: "portal-user", iss: "operan-tenant-control-plane",
    role: "admin", roles: ["admin"],
    exp: Math.floor(Date.now() / 1000) + 14400,
  }));
  const enc = new TextEncoder();
  const input = enc.encode(`${header}.${payload}`);
  let sig;
  if (crypto.subtle) {
    const key = await crypto.subtle.importKey("raw", enc.encode(secret),
      {name: "HMAC", hash: "SHA-256"}, false, ["sign"]);
    sig = await crypto.subtle.sign("HMAC", key, input);
  } else {
    sig = hmacSha256(enc.encode(secret), input);
  }
  return `${header}.${payload}.${b64url(sig)}`;
}

export async function api(method, url, body) {
  const res = await fetch(url, {
    method,
    headers: {"Authorization": "Bearer " + session.jwt, "X-Tenant-ID": session.tenant,
              "Content-Type": "application/json"},
    body: body ? JSON.stringify(body) : undefined,
  });
  let data = null;
  try { data = await res.json(); } catch (_) {}
  return {ok: res.ok, status: res.status, data};
}

export const get = (url) => api("GET", url);
export const post = (url, body) => api("POST", url, body);
export const patch = (url, body) => api("PATCH", url, body);
export const del = (url) => api("DELETE", url);
