import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./style.css";

function App() {
  const [status, setStatus] = useState(null);
  const [path, setPath] = useState("");
  const [prompt, setPrompt] = useState("");
  const [image, setImage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function api(url, body) {
    setBusy(true);
    setError("");
    try {
      const response = await fetch(url, {
        method: body ? "POST" : "GET",
        headers: { "content-type": "application/json" },
        body: body && JSON.stringify(body),
      });
      const value = await response.json();
      if (!response.ok) {
        throw new Error(value.error?.message || `request failed (${response.status})`);
      }
      return value;
    } catch (err) {
      setError(err.message);
      throw err;
    } finally {
      setBusy(false);
    }
  }

  async function refresh() {
    setStatus(await api("/v1/malina/models"));
  }

  useEffect(() => {
    refresh().catch(() => {});
  }, []);

  async function load() {
    try {
      await api("/v1/malina/models/load", { model_path: path });
      await refresh();
    } catch {}
  }

  async function unload() {
    try {
      await api("/v1/malina/models/unload", {});
      await refresh();
    } catch {}
  }

  async function generate() {
    try {
      const value = await api("/v1/images/generations", { prompt });
      setImage(value.data[0].b64_json);
    } catch {}
  }

  return (
    <main>
      <h1>Malina</h1>
      <p>{status?.loaded ? `Loaded: ${status.model.model_path}` : "No model loaded"}</p>
      <section>
        <input value={path} onChange={(event) => setPath(event.target.value)} placeholder="Model checkpoint path" />
        <button disabled={busy} onClick={load}>Load</button>
        <button disabled={busy} onClick={unload}>Unload</button>
      </section>
      <section>
        <textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder="Image prompt" />
        <button disabled={busy} onClick={generate}>Generate</button>
      </section>
      <p className="error">{error}</p>
      {image && <img src={`data:image/png;base64,${image}`} alt={prompt || "Generated image"} />}
    </main>
  );
}

createRoot(document.getElementById("root")).render(<App />);
