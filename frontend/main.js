// Git Update Zipper — frontend logic (two-pane layout)

let repoLoaded = false;
let currentBranch = "";
let changedFiles = []; // { path, status }
let selectedIdx = null; // currently viewed file, index into changedFiles

const el = (id) => document.getElementById(id);
const getMode = () => el("modeSelect").value;

// ── Add repo (GitHub Desktop style) ─────────────────────────────────────

el("addRepoBtn").onclick = async () => {
  const folder = await window.go.main.App.PickFolder();
  if (folder) loadRepo(folder);
};

el("browseOutputBtn").onclick = async () => {
  const folder = await window.go.main.App.PickFolder();
  if (folder) el("outputDir").value = folder;
};

// ── Load a repo by path ─────────────────────────────────────────────────

async function loadRepo(path) {
  const result = await window.go.main.App.SetRepo(path);
  if (result.ok) {
    currentBranch = result.branch;
    repoLoaded = true;

    const pill = el("branchPill");
    pill.textContent = `⎇ ${currentBranch}`;
    pill.style.display = "inline-block";

    // Show current repo at top
    const cur = el("currentRepo");
    cur.style.display = "flex";
    el("currentRepoName").textContent = path.split(/[\\/]/).pop();
    el("currentRepoPath").textContent = path;

    logLine(`Repo loaded: ${path} (${currentBranch})`);
    loadCommits();
    loadRecentPaths();
  } else {
    logLine(`Failed to load: ${result.error}`);
  }
}

// ── Recent paths ────────────────────────────────────────────────────────

async function loadRecentPaths() {
  const paths = await window.go.main.App.GetRecentPaths();
  const list = el("recentList");
  const empty = el("recentEmpty");

  if (!paths || paths.length === 0) {
    list.innerHTML = "";
    empty.style.display = "block";
    return;
  }

  empty.style.display = "none";
  list.innerHTML = "";

  paths.forEach((p) => {
    const item = document.createElement("div");
    item.className = "recent-item" + (repoLoaded && p.path === el("currentRepoPath").textContent ? " active" : "");
    const name = p.path.split(/[\\/]/).pop() || p.path;

    item.innerHTML = `
      <span class="recent-icon">⌗</span>
      <div class="recent-info">
        <div class="recent-name">${esc(name)}</div>
        <div class="recent-path-sub">${esc(p.path)}</div>
      </div>
      <button class="recent-remove" data-path="${esc(p.path)}" title="Remove">×</button>`;

    item.onclick = async (e) => {
      if (e.target.classList.contains("recent-remove")) return;
      loadRepo(p.path);
    };
    list.appendChild(item);
  });

  // Wire remove buttons
  list.querySelectorAll(".recent-remove").forEach(btn => {
    btn.onclick = async (e) => {
      e.stopPropagation();
      await window.go.main.App.RemoveRecentPath(btn.dataset.path);
      loadRecentPaths();
    };
  });
}

// ── Mode switching ──────────────────────────────────────────────────────

el("modeSelect").onchange = () => {
  el("rangePicker").style.display = getMode() === "range" ? "block" : "none";
  if (getMode() === "range") loadCommits();
};

// ── Commit picker ───────────────────────────────────────────────────────

el("loadCommitsBtn").onclick = loadCommits;

async function loadCommits() {
  if (!repoLoaded) return;
  try {
    const commits = await window.go.main.App.GetRecentCommits(30);
    const from = el("fromCommit");
    const to = el("toCommit");
    from.innerHTML = "";
    to.innerHTML = "";

    commits.forEach((c) => {
      const short = c.sha.substring(0, 7);
      const label = `${short} — ${c.message.substring(0, 44)}`;
      from.add(new Option(label, c.sha));
      to.add(new Option(label, c.sha));
    });
    if (commits.length >= 2) {
      from.selectedIndex = 1;
      to.selectedIndex = 0;
    }
  } catch (err) {
    logLine(`Failed to load commits: ${err}`);
  }
}

// ── Scan changes ────────────────────────────────────────────────────────

el("scanBtn").onclick = async () => {
  if (!repoLoaded) {
    logLine("Load a repo first — click one from the recent list or use + to add.");
    return;
  }

  const mode = getMode();
  let fromSHA = "", toSHA = "";
  if (mode === "range") {
    fromSHA = el("fromCommit").value;
    toSHA = el("toCommit").value;
  }

  try {
    changedFiles = await window.go.main.App.GetChangedFiles(mode, fromSHA, toSHA);
    selectedIdx = null;
    renderFileList();
    renderEmptyDiff();
    logLine(`Found ${changedFiles.length} changed files`);
  } catch (err) {
    logLine(`Error: ${err}`);
    el("fileArea").style.display = "none";
    el("sidebarEmpty").style.display = "block";
  }
};

// ── Render file list (sidebar) ──────────────────────────────────────────

function renderFileList() {
  const area = el("fileArea");
  const list = el("fileList");
  const count = el("fileCount");
  const empty = el("sidebarEmpty");

  if (changedFiles.length === 0) {
    area.style.display = "none";
    empty.style.display = "block";
    empty.querySelector("p").textContent = "No changes found.";
    return;
  }

  empty.style.display = "none";
  area.style.display = "flex";
  count.textContent = `${changedFiles.length} changed file${changedFiles.length === 1 ? "" : "s"}`;

  list.innerHTML = "";

  changedFiles.forEach((f, i) => {
    const row = document.createElement("div");
    row.className = "file-row";
    row.dataset.idx = i;

    row.innerHTML = `
      <input type="checkbox" class="file-check" data-idx="${i}" checked />
      <span class="file-status status-${f.status}">${f.status}</span>
      <span class="file-path">${esc(f.path)}</span>`;

    row.onclick = (e) => {
      if (e.target.tagName === "INPUT") return;
      selectFile(i);
    };
    row.querySelector(".file-check").onclick = (e) => e.stopPropagation();

    list.appendChild(row);
  });

  el("selectAllBtn").onclick = () => {
    list.querySelectorAll(".file-check").forEach((cb) => (cb.checked = true));
  };
  el("deselectAllBtn").onclick = () => {
    list.querySelectorAll(".file-check").forEach((cb) => (cb.checked = false));
  };
}

// ── Diff viewer (main pane) ─────────────────────────────────────────────

function renderEmptyDiff() {
  selectedIdx = null;
  el("diffHeader").style.display = "none";
  el("diffPane").innerHTML = `
    <div class="main-empty">
      <div class="main-empty-icon">⌗</div>
      <p>Select a file to view its diff</p>
      <p class="main-empty-sub">${changedFiles.length} file${changedFiles.length === 1 ? "" : "s"} changed — click one on the left</p>
    </div>`;
}

async function selectFile(idx) {
  selectedIdx = idx;
  const f = changedFiles[idx];

  document.querySelectorAll(".file-row").forEach((r) =>
    r.classList.toggle("selected", parseInt(r.dataset.idx) === idx)
  );

  const header = el("diffHeader");
  header.style.display = "flex";
  header.innerHTML = `
    <span class="file-status status-${f.status}">${f.status}</span>
    <span id="diffFilePath">${esc(f.path)}</span>`;

  const pane = el("diffPane");
  pane.innerHTML = `<div class="diff-loading">Loading diff…</div>`;

  try {
    const mode = getMode();
    let fromSHA = "", toSHA = "";
    if (mode === "range") {
      fromSHA = el("fromCommit").value;
      toSHA = el("toCommit").value;
    }
    const diffText = await window.go.main.App.GetFileDiff(mode, fromSHA, toSHA, f.path);
    pane.innerHTML = renderDiff(diffText);
  } catch (err) {
    pane.innerHTML = `<div class="diff-err">Error loading diff: ${esc(String(err))}</div>`;
  }
}

function renderDiff(text) {
  if (!text || text.trim() === "") {
    return `<div class="diff-empty">(empty file / no changes)</div>`;
  }

  const lines = text.split("\n");
  let html = `<div class="diff-view">`;
  for (const line of lines) {
    let cls = "";
    if (line.startsWith("+") && !line.startsWith("+++")) cls = "diff-add";
    else if (line.startsWith("-") && !line.startsWith("---")) cls = "diff-del";
    else if (line.startsWith("@@")) cls = "diff-hunk";
    else if (
      line.startsWith("diff ") ||
      line.startsWith("index ") ||
      line.startsWith("---") ||
      line.startsWith("+++")
    )
      cls = "diff-meta";

    html += `<div class="diff-line ${cls}">${esc(line)}</div>`;
  }
  html += `</div>`;
  return html;
}

// ── Generate Zip ────────────────────────────────────────────────────────

el("generateBtn").onclick = async () => {
  const outputDir = el("outputDir").value.trim();
  if (!outputDir) {
    el("outputDir").focus();
    return;
  }

  const mode = getMode();
  let fromSHA = "", toSHA = "";
  if (mode === "range") {
    fromSHA = el("fromCommit").value;
    toSHA = el("toCommit").value;
  }

  const checks = document.querySelectorAll(".file-check:checked");
  const selected = [];
  checks.forEach((cb) => selected.push(changedFiles[parseInt(cb.dataset.idx)].path));

  if (selected.length === 0) {
    logLine("Select at least one file first.");
    return;
  }

  const btn = el("generateBtn");
  btn.disabled = true;
  btn.textContent = "Generating…";

  const banner = el("resultBanner");

  try {
    const res = await window.go.main.App.GenerateZip(mode, fromSHA, toSHA, outputDir, selected);
    banner.style.display = "block";
    if (res.ok && res.count > 0) {
      banner.innerHTML = `
        <div class="result-ok">
          <button class="close-banner">×</button>
          <strong>${res.count} files zipped</strong>
          <div class="result-name">${esc(res.zip_name)}</div>
          <div class="result-files">${res.files.map(esc).join("<br>")}</div>
          <button class="open-btn" data-path="${esc(res.zip_path)}">Show in folder</button>
        </div>`;
      banner.querySelector(".open-btn").onclick = () =>
        window.go.main.App.OpenFolder(outputDir);
      logLine(`Zip created: ${res.zip_name} (${res.count} files)`);
    } else if (res.ok) {
      banner.innerHTML = `<div class="result-empty">No files selected.</div>`;
    } else {
      banner.innerHTML = `<div class="result-err"><button class="close-banner">×</button>${esc(res.error)}</div>`;
      logLine(`Error: ${res.error}`);
    }
    banner.querySelector(".close-banner")?.addEventListener("click", () => {
      banner.style.display = "none";
    });
  } catch (err) {
    banner.style.display = "block";
    banner.innerHTML = `<div class="result-err">${esc(String(err))}</div>`;
    logLine(`Error: ${err}`);
  }

  btn.disabled = false;
  btn.textContent = "Generate Zip";
};

// ── Log (single-line status bar) ────────────────────────────────────────

function logLine(text) {
  const strip = el("logStrip");
  strip.textContent = `${new Date().toLocaleTimeString()} — ${text}`;
  strip.title = text;
}

// ── Helpers ─────────────────────────────────────────────────────────────

function esc(s) {
  if (!s) return "";
  const d = document.createElement("div");
  d.textContent = s;
  return d.innerHTML;
}

// ── Init ────────────────────────────────────────────────────────────────

(async () => {
  const ok = await window.go.main.App.GitAvailable();
  const gitPill = el("gitStatus");
  if (ok) {
    gitPill.textContent = "git ready";
    gitPill.className = "pill pill-ok";
  } else {
    gitPill.textContent = "git not found";
    gitPill.className = "pill pill-err";
    logLine("git not found on PATH — app won't work without git.");
    el("setRepoBtn").disabled = true;
    el("scanBtn").disabled = true;
  }
  loadRecentPaths();
})();
