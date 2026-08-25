import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../api.js";

const STAGES = ["saved", "applied", "screen", "interview", "offer", "closed"];

function pct(n) {
  return n == null ? "—" : `${Math.round(n * 100)}%`;
}

const BAND_LABEL = {
  strong: "strong match",
  possible: "possible match",
  stretch: "stretch",
  excluded: "excluded by a hard gate",
};

export default function JobDetail() {
  const { id } = useParams();
  const nav = useNavigate();
  const [job, setJob] = useState(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function load() {
    try {
      setJob(await api.job(id));
    } catch (e) {
      setErr(e.message);
    }
  }
  useEffect(() => { load(); }, [id]);

  async function save() {
    setBusy(true);
    setErr("");
    try {
      const app = await api.saveJob(id);
      await load();
      nav("/tracker");
      return app;
    } catch (e) {
      setErr(e.message);
    } finally {
      setBusy(false);
    }
  }

  async function move(status) {
    if (!job?.application?.id) {
      await save();
      return;
    }
    setBusy(true);
    try {
      await api.moveApp(job.application.id, status);
      await load();
    } catch (e) {
      setErr(e.message);
    } finally {
      setBusy(false);
    }
  }

  if (!job && !err) return <main><p>Loading…</p></main>;
  if (!job) return <main><p className="err">{err}</p></main>;

  const score = job.score;
  const band = score?.band || "unscored";

  return (
    <main className="detail">
      <Link to="/" className="back">← Jobs</Link>
      <h1>{job.title}</h1>
      <p className="sub">{job.company} · {job.location || "—"} · {job.source}</p>
      <div className="row">
        <div className="score big" data-band={band}>{score ? pct(score.composite) : "…"}</div>
        <div>
          <p>
            <strong>Match</strong>{" "}
            {score ? `${pct(score.composite)} · ${BAND_LABEL[band] || band}` : "not scored yet — the agent hasn't run against this profile (make agent, or make embed for one pass)"}
          </p>
          {band === "excluded" && (
            <p className="err">Excluded by a hard gate — a must-have skill, seniority, or location preference didn't match. The sub-scores below still show why.</p>
          )}
          {score && (
            <p className="chips">
              <span className="pipe">skill coverage {pct(score.skill_cov)}</span>
              <span className="pipe">semantic {pct(score.semantic)}</span>
              <span className="pipe">seniority {pct(score.seniority_fit)}</span>
              <span className="pipe">location {pct(score.location_fit)}</span>
              <span className="pipe">recency {pct(score.recency)}</span>
            </p>
          )}
          <p><strong>Matched</strong> {(score?.matched_skills || []).join(", ") || "—"}</p>
          <p><strong>Job asks, not on resume</strong> {(score?.missing_skills || []).join(", ") || "—"}</p>
        </div>
      </div>
      <div className="actions">
        <a className="btn ghost" href={job.source_url} target="_blank" rel="noreferrer">Open posting</a>
        {!job.application && <button className="btn" disabled={busy} onClick={save}>Save to tracker</button>}
        {job.application && (
          <label>
            Pipeline
            <select value={job.application.status} disabled={busy} onChange={(e) => move(e.target.value)}>
              {STAGES.map((s) => <option key={s} value={s}>{s}</option>)}
            </select>
          </label>
        )}
      </div>
      {err && <p className="err">{err}</p>}
      <article className="desc">{job.description_md || "No description stored."}</article>
    </main>
  );
}
