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
    band: top.score?.band ?? null,
    score: top.score?.composite ?? 0,
  };
}

function pipeline(profile, jobs, phase) {
  const resume = profile?.latest_resume;
  const scored = (jobs || []).some((j) => j.score != null);
  const stored = Boolean(resume) || phase === "uploading" || phase === "waiting";
  const parsed = resume?.status === "done";
  const failed = resume?.status === "error";
  const embedded = Boolean(profile?.has_embedding);
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
      hint: embedded ? "Vector ready for the semantic sub-score" : "Same agent pass writes the profile vector",
      state: embedded ? "done" : parsed ? "active" : "todo",
    },
    {
      id: "scored",
      label: "Jobs scored",
      hint: scored ? "List ordered by composite score (skill coverage, semantic, seniority, location, recency)" : "Agent's scoring pass hasn't run against this profile yet",
      state: scored ? "done" : embedded ? "active" : "todo",
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
        const scored = st === "done" && p.has_embedding && nextJobs.some((j) => j.score != null);
        if (scored) {
          const before = beforeRef.current;
          const after = snapshot(nextJobs);
          if (before && after && before.id !== after.id) {
            setNotice(
              `Matching updated. Was “${before.title}” (${before.band || "unscored"}). Now “${after.title}” (${after.band || "unscored"} ${pct(after.score)}).`,
            );
          } else {
            setNotice("Matching updated from this resume. List is ranked by composite score.");
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
      setNotice("Skill list saved. Re-scoring waits until the next agent pass (make embed or make agent).");
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
  const scored = jobs.some((j) => j.score != null);
  const busy = phase === "uploading" || phase === "waiting";

  return (
    <main>
      <section className="pipeline" aria-live="polite">
        <div className="pipeline-head">
          <h2>Match pipeline</h2>
          <p>
            {phase === "uploading" && "Uploading resume…"}
            {phase === "waiting" && "Waiting for the local agent to parse and embed. The job list refreshes when that finishes."}
            {phase === "idle" && (scored
              ? "Match % is a composite of skill coverage, semantic similarity, seniority, location, and recency — with a strong/possible/stretch band."
              : "Not scored yet. Upload a resume, or run the agent, to see match scores.")}
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
        {jobs.length} jobs · ranked by composite score
        {busy ? " · watching for agent…" : ""}
      </p>
      <ul className="cards">
        {jobs.map((j) => {
          const band = j.score?.band || "unscored";
          return (
            <li key={j.id}>
              <Link to={`/jobs/${j.id}`} className="card">
                <div className="score" data-band={band}>
                  {j.score ? pct(j.score.composite) : "…"}
                </div>
                <div>
                  <h2>{j.title}</h2>
                  <p>{j.company} · {j.location || "—"} · {j.source}</p>
                  <p className="chips">
                    {j.score && <span className="pipe">{band}</span>}
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
