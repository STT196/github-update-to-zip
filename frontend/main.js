// Git Update Zipper — frontend with diff viewer

let repoLoaded = false;
let currentBranch = "";
let changedFiles = []; // { path, status }
let expandedFile = null; // currently viewed diff

// ── Set repo ────────────────────────────────────────────────────────────

document.getElementById("setRepoBtn").onclick = async () => {
  const path = document.getElementById("repoPath").value.trim();
  const status = document.getElementById("repoStatus");
  if (!path) return;

  const result = await window.go.main.App.SetRepo(path);
  if (result.ok) {
    currentBranch = result.branch;
    status.textContent = `✓ Loaded — branch: ${currentBranch}`;
    status.className = "hint ok";
    repoLoaded = true;
    logLine(`Repo loaded: ${path} (${currentBranch})`);
    loadCommits();
  } else {
    status.textContent = `✗ ${result.error}`;
    status.className = "hint err";
    repoLoaded = false;
  }
};

// ── Mode switching ──────────────────────────────────────────────────────

document.querySelectorAll("input[name='mode']").forEach(r => {
  r.onchange = () => {
    document.getElementById("rangePicker").style.display = r.value === "range" ? "block" : "none";
    if (r.value === "range") loadCommits();
  };
});

// ── Commit picker ───────────────────────────────────────────────────────

document.getElementById("loadCommitsBtn").onclick = loadCommits;

async function loadCommits() {
  if (!repoLoaded) return;
  try {
    const commits = await window.go.main.App.GetRecentCommits(30);
    const from = document.getElementById("fromCommit");
    const to = document.getElementById("toCommit");
    from.innerHTML = "";
    to.innerHTML = "";

    commits.forEach((c) => {
      const short = c.sha.substring(0, 7);
      const label = `${short} — ${c.message.substring(0, 60)}`;
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

document.getElementById("scanBtn").onclick = async () => {
  if (!repoLoaded) { alert("Load a repo first."); return; }

  const mode = document.querySelector("input[name='mode']:checked").value;
  let fromSHA = "", toSHA = "";
  if (mode === "range") {
    fromSHA = document.getElementById("fromCommit").value;
    toSHA = document.getElementById("toCommit").value;
  }

  try {
    changedFiles = await window.go.main.App.GetChangedFiles(mode, fromSHA, toSHA);
    renderFileList();
    logLine(`Found ${changedFiles.length} changed files`);
  } catch (err) {
    logLine(`Error: ${err}`);
    document.getElementById("fileArea").style.display = "none";
  }
};

// ── Render file list ────────────────────────────────────────────────────

function renderFileList() {
  const area = document.getElementById("fileArea");
  const list = document.getElementById("fileList");
  const count = document.getElementById("fileCount");

  if (changedFiles.length === 0) {
    area.style.display = "none";
    document.getElementById("result").innerHTML = `<div class="result-empty">No changes found.</div>`;
    return;
  }

  area.style.display = "block";
  count.textContent = `${changedFiles.length} changed files`;

  list.innerHTML = "";
  expandedFile = null;

  changedFiles.forEach((f, i) => {
    const row = document.createElement("div");
    row.className = "file-row";
    const statusClass = `status-${f.status}`;

    row.innerHTML = `
      <div class="file-main" data-idx="${i}">
        <input type="checkbox" class="file-check" data-idx="${i}" checked />
        <span class="file-status ${statusClass}">${f.status}</span>
        <span class="file-path">${esc(f.path)}</span>
        <span class="file-expand">▸</span>
      </div>
      <div class="file-diff" id="diff-${i}" style="display:none">
        <div class="diff-loading">Loading diff...</div>
      </div>`;

    // Toggle diff on click (not on checkbox)
    row.querySelector(".file-main").onclick = async (e) => {
      if (e.target.tagName === "INPUT") return;
      await toggleDiff(i, f.path);
    };

    // Prevent checkbox click from toggling diff
    row.querySelector(".file-check").onclick = (e) => e.stopPropagation();

    list.appendChild(row);
  });

  // Select all / none
  document.getElementById("selectAllBtn").onclick = () => {
    list.querySelectorAll(".file-check").forEach(cb => cb.checked = true);
  };
  document.getElementById("deselectAllBtn").onclick = () => {
    list.querySelectorAll(".file-check").forEach(cb => cb.checked = false);
  };
}

// ── Diff viewer ─────────────────────────────────────────────────────────

async function toggleDiff(idx, filePath) {
  const diffDiv = document.getElementById(`diff-${idx}`);
  const expandIcon = document.querySelector(`.file-main[data-idx="${idx}"] .file-expand`);

  // Collapse if already open
  if (expandedFile === idx) {
    diffDiv.style.display = "none";
    expandIcon.textContent = "▸";
    expandedFile = null;
    return;
  }

  // Collapse previous
  if (expandedFile !== null) {
    const prevDiff = document.getElementById(`diff-${expandedFile}`);
    const prevIcon = document.querySelector(`.file-main[data-idx="${expandedFile}"] .file-expand`);
    if (prevDiff) prevDiff.style.display = "none";
    if (prevIcon) prevIcon.textContent = "▸";
  }

  // Expand this one
  expandedFile = idx;
  diffDiv.style.display = "block";
  expandIcon.textContent = "▾";

  // Fetch diff if not already loaded
  if (diffDiv.querySelector(".diff-loading")) {
    try {
      const mode = document.querySelector("input[name='mode']:checked").value;
      let fromSHA = "", toSHA = "";
      if (mode === "range") {
        fromSHA = document.getElementById("fromCommit").value;
        toSHA = document.getElementById("toCommit").value;
      }
      const diffText = await window.go.main.App.GetFileDiff(mode, fromSHA, toSHA, filePath);
      diffDiv.innerHTML = renderDiff(diffText);
    } catch (err) {
      diffDiv.innerHTML = `<div class="diff-err">Error loading diff: ${esc(String(err))}</div>`;
    }
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
    else if (line.startsWith("diff ") || line.startsWith("index ") ||
             line.startsWith("---") || line.startsWith("+++")) cls = "diff-meta";

    html += `<div class="diff-line ${cls}">${esc(line)}</div>`;
  }
  html += `</div>`;
  return html;
}

// ── Generate Zip ────────────────────────────────────────────────────────

document.getElementById("generateBtn").onclick = async () => {
  const outputDir = document.getElementById("outputDir").value.trim();
  if (!outputDir) { alert("Enter an output folder."); return; }

  const mode = document.querySelector("input[name='mode']:checked").value;
  let fromSHA = "", toSHA = "";
  if (mode === "range") {
    fromSHA = document.getElementById("fromCommit").value;
    toSHA = document.getElementById("toCommit").value;
  }

  // Collect selected files
  const checks = document.querySelectorAll(".file-check:checked");
  const selected = [];
  checks.forEach(cb => selected.push(changedFiles[parseInt(cb.dataset.idx)].path));

  if (selected.length === 0) { alert("Select at least one file."); return; }

  const btn = document.getElementById("generateBtn");
  btn.disabled = true;
  btn.textContent = "Generating...";

  const result = document.getElementById("result");

  try {
    const res = await window.go.main.App.GenerateZip(mode, fromSHA, toSHA, outputDir, selected);
    if (res.ok && res.count > 0) {
      result.innerHTML = `
        <div class="result-ok">
          <strong>✓ ${res.count} files zipped</strong>
          <div class="result-name">${esc(res.zip_name)}</div>
          <div class="result-files">${res.files.map(esc).join("<br>")}</div>
          <button class="open-btn" data-path="${esc(res.zip_path)}">Open folder</button>
        </div>`;
      result.querySelector(".open-btn").onclick = () =>
        window.go.main.App.OpenFolder(outputDir);
      logLine(`Zip created: ${res.zip_name} (${res.count} files)`);
    } else if (res.ok) {
      result.innerHTML = `<div class="result-empty">No files selected.</div>`;
    } else {
      result.innerHTML = `<div class="result-err">${esc(res.error)}</div>`;
      logLine(`Error: ${res.error}`);
    }
  } catch (err) {
    result.innerHTML = `<div class="result-err">${esc(String(err))}</div>`;
    logLine(`Error: ${err}`);
  }

  btn.disabled = false;
  btn.textContent = "Generate Zip";
};

// ── Log ─────────────────────────────────────────────────────────────────

function logLine(text) {
  const strip = document.getElementById("logStrip");
  const line = document.createElement("div");
  line.className = "log-line";
  line.textContent = `[${new Date().toLocaleTimeString()}] ${text}`;
  strip.prepend(line);
  while (strip.children.length > 20) strip.removeChild(strip.lastChild);
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
  if (!ok) {
    logLine("⚠ git not found on PATH — app won't work without git.");
    document.getElementById("setRepoBtn").disabled = true;
    document.getElementById("scanBtn").disabled = true;
  }
})();
