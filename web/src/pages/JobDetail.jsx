import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../api.js";

const STAGES = ["saved", "applied", "screen", "interview", "offer", "closed"];

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

  const match = Math.round((job.score?.coverage || 0) * 100);

  return (
    <main className="detail">
      <Link to="/" className="back">← Jobs</Link>
      <h1>{job.title}</h1>
      <p className="sub">{job.company} · {job.location || "—"} · {job.source}</p>
      <div className="row">
        <div className="score big" data-band={match >= 50 ? "high" : match >= 25 ? "mid" : "low"}>{match}%</div>
        <div>
          <p><strong>Skill match</strong> {match}%{job.score?.semantic != null ? ` · similarity ${Math.round(job.score.semantic * 100)}%` : ""}</p>
          <p><strong>Matched</strong> {(job.score?.matched_skills || []).join(", ") || "—"}</p>
          <p><strong>Job asks, not on resume</strong> {(job.score?.missing_skills || []).join(", ") || "—"}</p>
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
