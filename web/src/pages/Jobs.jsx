import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api.js";

function pct(n) {
  return `${Math.round((n || 0) * 100)}%`;
}

function snapshot(jobs) {
  const top = jobs[0];
  if (!top) return null;
  return {
    id: top.id,
    title: top.title,
    company: top.company,
    mode: top.score?.semantic != null ? "semantic" : "keyword",
    score: top.score?.semantic ?? top.score?.coverage ?? 0,
  };
}

function pipeline(profile, jobs, phase) {
  const resume = profile?.latest_resume;
  const semantic = (jobs || []).some((j) => j.score?.semantic != null);
  const stored = Boolean(resume) || phase === "uploading" || phase === "waiting";
  const parsed = resume?.status === "done";
  const failed = resume?.status === "error";
  const embedded = Boolean(profile?.has_embedding);
  const ranked = parsed && embedded && semantic;
  return [
    {
      id: "stored",
      label: "Resume stored",
      hint: "File saved locally. Nothing is sent to a cloud model.",
      state: phase === "uploading" ? "active" : stored ? "done" : "todo",
    },
    {
      id: "parsed",
      label: "Skills extracted",
      hint: failed
        ? (resume.error || "Parse failed")
        : parsed
          ? `${(profile.skills || []).length} skills from the resume`
          : "Agent reads the file (make embed or make agent)",
      state: failed ? "error" : parsed ? "done" : stored || phase === "waiting" ? "active" : "todo",
    },
    {
      id: "embedded",
      label: "Profile embedded",
      hint: embedded ? "Vector ready for similarity search" : "Same agent pass writes the profile vector",
      state: embedded ? "done" : parsed ? "active" : "todo",
    },
    {
      id: "ranked",
      label: "Jobs re-ranked",
      hint: ranked ? "List ordered by semantic similarity" : "Falls back to keyword overlap until vectors exist",
      state: ranked ? "done" : embedded ? "active" : "todo",
    },
  ];
}

export default function Jobs() {
  const [jobs, setJobs] = useState([]);
  const [skills, setSkills] = useState("");
  const [profile, setProfile] = useState(null);
  const [err, setErr] = useState("");
  const [saving, setSaving] = useState(false);
  const [phase, setPhase] = useState("idle");
  const [notice, setNotice] = useState("");
  const beforeRef = useRef(null);
  const watchStarted = useRef(0);

  async function refresh() {
    const [j, p] = await Promise.all([api.jobs(), api.profile()]);
    setJobs(j || []);
    setProfile(p);
    setSkills((p.skills || []).join(", "));
    return { jobs: j || [], profile: p };
  }

  useEffect(() => {
    refresh().then(({ profile: p }) => {
      if (p?.latest_resume?.status === "pending") {
        setPhase("waiting");
        watchStarted.current = Date.now();
      }
    }).catch((e) => setErr(e.message));
  }, []);

  useEffect(() => {
    if (phase !== "waiting") return undefined;
    let stop = false;
    async function tick() {
      if (stop) return;
      try {
        const { jobs: nextJobs, profile: p } = await refresh();
        const st = p?.latest_resume?.status;
        if (st === "error") {
          setPhase("idle");
          setErr(p.latest_resume.error || "Resume parse failed");
          setNotice("");
          return;
        }
        const ranked = st === "done" && p.has_embedding && nextJobs.some((j) => j.score?.semantic != null);
        if (ranked) {
          const before = beforeRef.current;
          const after = snapshot(nextJobs);
          if (before && after && before.id !== after.id) {
            setNotice(
              `Matching updated. Was “${before.title}” (${before.mode} ${pct(before.score)}). Now “${after.title}” (semantic ${pct(after.score)}).`,
            );
          } else {
            setNotice("Matching updated from this resume. List is ranked by semantic similarity.");
          }
          setPhase("idle");
          return;
        }
        if (st === "done" && p.has_embedding) {
          setNotice("Profile is embedded. Re-ranking jobs…");
        }
        if (Date.now() - watchStarted.current > 90000) {
          setNotice("Still waiting on the agent. In a terminal: EMBED_BACKEND=fake make embed  (or make agent). This page will pick up the result.");
        }
      } catch (e) {
        if (!stop) setErr(e.message);
      }
    }
    tick();
    const id = setInterval(tick, 1000);
    return () => {
      stop = true;
      clearInterval(id);
    };
  }, [phase]);

  async function saveSkills(e) {
    e.preventDefault();
    setSaving(true);
    setErr("");
    try {
      const list = skills.split(",").map((s) => s.trim()).filter(Boolean);
      await api.saveProfile(list);
      await refresh();
      setNotice("Skill list saved. Keyword rank updated; semantic rank waits until the next embed pass.");
    } catch (e) {
      setErr(e.message);
    } finally {
      setSaving(false);
    }
  }

  async function onUpload(e) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setErr("");
    setNotice("");
    beforeRef.current = snapshot(jobs);
    setPhase("uploading");
    try {
      await api.uploadResume(file);
      watchStarted.current = Date.now();
      setPhase("waiting");
    } catch (e) {
      setErr(e.message);
      setPhase("idle");
    }
  }

  const steps = pipeline(profile, jobs, phase);
  const semantic = jobs.some((j) => j.score?.semantic != null);
  const busy = phase === "uploading" || phase === "waiting";

  return (
    <main>
      <section className="pipeline" aria-live="polite">
        <div className="pipeline-head">
          <h2>Match pipeline</h2>
          <p>
            {phase === "uploading" && "Uploading resume…"}
            {phase === "waiting" && "Waiting for the local agent to parse and embed. The job list refreshes when that finishes."}
            {phase === "idle" && (semantic
              ? "Match % is skill overlap from the resume. Similarity is a secondary signal, not the headline score."
              : "Match % is skill overlap. Upload a resume to refresh the skill list.")}
          </p>
        </div>
        <ol>
          {steps.map((s, i) => (
            <li key={s.id} data-state={s.state}>
              <span className="step-n">{s.state === "done" ? "✓" : i + 1}</span>
              <div>
                <strong>{s.label}</strong>
                <p>{s.hint}</p>
              </div>
            </li>
          ))}
        </ol>
        {notice && <p className="notice">{notice}</p>}
      </section>

      <form className="skills" onSubmit={saveSkills}>
        <label>
          Skill list (keyword override — does not re-embed)
          <input value={skills} onChange={(e) => setSkills(e.target.value)} placeholder="kubernetes, terraform, aws" />
        </label>
        <button type="submit" disabled={saving || busy}>{saving ? "Saving…" : "Re-rank keywords"}</button>
      </form>
      <div className="skills upload">
        <label>
          Resume (PDF or DOCX) — stored locally, parsed by the agent
          <input type="file" accept=".pdf,.docx,application/pdf" onChange={onUpload} disabled={busy} />
        </label>
      </div>
      {err && <p className="err">{err}</p>}
      <p className="meta">
        {jobs.length} jobs · ranked by skill overlap
        {busy ? " · watching for agent…" : ""}
      </p>
      <ul className="cards">
        {jobs.map((j) => {
          const match = j.score?.coverage || 0;
          return (
            <li key={j.id}>
              <Link to={`/jobs/${j.id}`} className="card">
                <div className="score" data-band={match >= 0.5 ? "high" : match >= 0.25 ? "mid" : "low"}>
                  {pct(match)}
                </div>
                <div>
                  <h2>{j.title}</h2>
                  <p>{j.company} · {j.location || "—"} · {j.source}</p>
                  <p className="chips">
                    {j.score?.semantic != null && <span className="pipe">sim {pct(j.score.semantic)}</span>}
                    {(j.score?.matched_skills || []).slice(0, 6).map((s) => <span key={s}>{s}</span>)}
                    {j.application && <span className="pipe">{j.application.status}</span>}
                  </p>
                </div>
              </Link>
            </li>
          );
        })}
      </ul>
    </main>
  );
}
