import { useMemo, useState } from "react";

const apiPath = "/api/admin/stats";

function classifyStatus(status) {
  if (status === null) {
    return "idle";
  }
  if (status === 401 || status === 403) {
    return "proxied";
  }
  if (status >= 200 && status < 300) {
    return "ok";
  }
  if (status >= 500) {
    return "backend";
  }
  return "unexpected";
}

function trimBody(value) {
  if (!value) {
    return "";
  }
  return value.length > 420 ? `${value.slice(0, 420)}...` : value;
}

export default function App() {
  const [check, setCheck] = useState({
    status: null,
    message: "Ready to test the CloudFront to admin API path.",
    body: ""
  });
  const [loading, setLoading] = useState(false);

  const result = useMemo(() => classifyStatus(check.status), [check.status]);

  async function testApiProxy() {
    setLoading(true);
    setCheck({
      status: null,
      message: `Calling ${apiPath} through the current origin...`,
      body: ""
    });

    try {
      const headers = {};
      const adminSecret = import.meta.env.VITE_ADMIN_SECRET;
      if (adminSecret) {
        headers["X-Admin-Secret"] = adminSecret;
      }

      const response = await fetch(apiPath, {
        credentials: "include",
        headers
      });
      const text = await response.text();

      setCheck({
        status: response.status,
        message:
          response.status === 401 || response.status === 403
            ? "Proxy reached the admin backend. Auth is blocking the test request as expected."
            : response.ok
              ? "Proxy reached the admin backend and returned a successful response."
              : "Proxy returned a response, but the status needs a quick look.",
        body: trimBody(text)
      });
    } catch (error) {
      setCheck({
        status: 0,
        message: "Request failed before receiving a backend response.",
        body: error instanceof Error ? error.message : String(error)
      });
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="shell">
      <section className="status-panel">
        <div className="eyebrow">Smatch admin web</div>
        <h1>Infrastructure check</h1>
        <p className="lede">
          Temporary React app for validating S3, CloudFront, Route53, TLS, and
          the admin API proxy path before the real admin UI ships.
        </p>

        <div className="checks">
          <div className="check-card">
            <span className="label">Static host</span>
            <strong>S3 private origin</strong>
            <span className="detail">Served through CloudFront only</span>
          </div>
          <div className="check-card">
            <span className="label">Domain</span>
            <strong>admin-sb.online</strong>
            <span className="detail">CloudFront alias with TLS</span>
          </div>
          <div className="check-card">
            <span className="label">API path</span>
            <strong>/api/*</strong>
            <span className="detail">Forwarded to the admin backend</span>
          </div>
        </div>

        <div className={`result result-${result}`}>
          <div>
            <span className="label">Latest proxy test</span>
            <strong>{check.status === null ? "Not run yet" : `HTTP ${check.status}`}</strong>
            <p>{check.message}</p>
          </div>
          <button type="button" onClick={testApiProxy} disabled={loading}>
            {loading ? "Testing..." : "Test API proxy"}
          </button>
        </div>

        {check.body ? (
          <pre className="response" aria-label="API response preview">
            {check.body}
          </pre>
        ) : null}
      </section>
    </main>
  );
}
