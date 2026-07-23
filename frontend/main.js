// Git Update Zipper — frontend logic (GitHub Desktop style sidebar)

let repoLoaded = false;
let currentBranch = "";
let changedFiles = []; // { path, status }
let selectedIdx = null; // currently viewed file, index into changedFiles
let recentPaths = []; // cached recent paths

const el = (id) => document.getElementById(id);
const getMode = () => el("modeSelect").value;

// ── Icons (inline SVG strings) ────────────────────────────────────────

const FolderIcon = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M9.2373 3.7002C10.4169 3.7002 11.5297 4.24779 12.249 5.18262L12.4424 5.43359H18C20.0987 5.43359 21.7998 7.13472 21.7998 9.2334V16.5C21.7998 18.5987 20.0987 20.2998 18 20.2998H6C3.90132 20.2998 2.2002 18.5987 2.2002 16.5V7.5C2.2002 5.40132 3.90132 3.7002 6 3.7002H9.2373ZM6 5.5C4.89543 5.5 4 6.39543 4 7.5V16.5C4 17.6046 4.89543 18.5 6 18.5H18C19.0357 18.5 19.887 17.7128 19.9893 16.7041L20 16.5V9.2334C20 8.19775 19.2128 7.34641 18.2041 7.24414L18 7.2334H12.0479L11.9326 7.22656C11.666 7.19561 11.4205 7.05812 11.2549 6.84277L10.8223 6.28027C10.4437 5.78834 9.85808 5.5 9.2373 5.5H6ZM16 9.59961C16.4971 9.59961 16.9004 10.0029 16.9004 10.5C16.9004 10.9971 16.4971 11.4004 16 11.4004H8C7.50294 11.4004 7.09961 10.9971 7.09961 10.5C7.09961 10.0029 7.50294 9.59961 8 9.59961H16Z" fill="currentColor"/></svg>`;

const LockIcon = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path fill-rule="evenodd" clip-rule="evenodd" d="M7 5C7 3.34315 8.34315 2 10 2H14C15.6569 2 17 3.34315 17 5V6H18C19.6569 6 21 7.34315 21 9V19C21 20.6569 19.6569 22 18 22H6C4.34315 22 3 20.6569 3 19V9C3 7.34315 4.34315 6 6 6H7V5ZM10 4C9.44772 4 9 4.44772 9 5V6H15V5C15 4.44772 14.5523 4 14 4H10ZM6 8C5.44772 8 5 8.44772 5 9V19C5 19.5523 5.44772 20 6 20H18C18.5523 20 19 19.5523 19 19V9C19 8.44772 18.5523 8 18 8H6Z" fill="currentColor"/></svg>`;

const GlobeIcon = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path fill-rule="evenodd" clip-rule="evenodd" d="M12 2C6.47715 2 2 6.47715 2 12C2 17.5228 6.47715 22 12 22C17.5228 22 22 17.5228 22 12C22 6.47715 17.5228 2 12 2ZM15.5 16.5C15.5 16.5 14.5 18 12 18C9.5 18 8.5 16.5 8.5 16.5C8.5 16.5 7 14.5 7 12C7 9.5 8.5 7.5 8.5 7.5C8.5 7.5 9.5 6 12 6C14.5 6 15.5 7.5 15.5 7.5C15.5 7.5 17 9.5 17 12C17 14.5 15.5 16.5 15.5 16.5ZM12 8C10.3431 8 9 9.79086 9 12C9 14.2091 10.3431 16 12 16C13.6569 16 15 14.2091 15 12C15 9.79086 13.6569 8 12 8Z" fill="currentColor"/></svg>`;

const DownloadIcon = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M12.0002 15.5C12.5525 15.5 13.0002 15.0523 13.0002 14.5V7.41421L14.2932 8.70711C14.6837 9.09763 15.3169 9.09763 15.7074 8.70711C16.0979 8.31658 16.0979 7.68342 15.7074 7.29289L12.7074 4.29289C12.3169 3.90237 11.6837 3.90237 11.2932 4.29289L8.29321 7.29289C7.90269 7.68342 7.90269 8.31658 8.29321 8.70711C8.68373 9.09763 9.3169 9.09763 9.70742 8.70711L11.0002 7.41421V14.5C11.0002 15.0523 11.4479 15.5 12.0002 15.5Z" fill="currentColor"/><path d="M4 16C4 15.4477 4.44772 15 5 15H7C7.55228 15 8 15.4477 8 16C8 16.5523 7.55228 17 7 17H5V19H19V17H17C16.4477 17 16 16.5523 16 16C16 15.4477 16.4477 15 17 15H19C19.5523 15 20 15.4477 20 16V20C20 20.5523 19.5523 21 19 21H5C4.44772 21 4 20.5523 4 20V16Z" fill="currentColor"/></svg>`;

// ── Add repo ────────────────────────────────────────────────────────────

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

    // Update current repo header
    el("currentRepoName").textContent = path.split(/[\\/]/).pop();

    logLine(`Repo loaded: ${path} (${currentBranch})`);
    loadCommits();
    loadRecentPaths();
  } else {
    logLine(`Failed to load: ${result.error}`);
  }
}

// ── Recent paths (GitHub Desktop grouped style) ───────────────────────

async function loadRecentPaths() {
  const paths = await window.go.main.App.GetRecentPaths();
  const list = el("recentList");
  const empty = el("recentEmpty");
  recentPaths = paths || [];

  if (!paths || paths.length === 0) {
    list.innerHTML = "";
    empty.style.display = "block";
    return;
  }

  empty.style.display = "none";
  list.innerHTML = "";

  // Group by parent directory (simulating org/user grouping)
  const groups = {};
  paths.forEach((p) => {
    const parts = p.path.split(/[\\/]/);
    const name = parts.pop() || p.path;
    const parent = parts.length > 0 ? parts[parts.length - 1] : "Other";
    
    if (!groups[parent]) groups[parent] = [];
    groups[parent].push({ ...p, name, parent });
  });

  // Render groups
  Object.keys(groups).forEach((groupName) => {
    const group = document.createElement("div");
    group.className = "repo-group";

    // Group header
    const header = document.createElement("div");
    header.className = "repo-group-header";
    header.innerHTML = `${FolderIcon}<span>${esc(groupName)}</span>`;
    group.appendChild(header);

    // Repo items
    groups[groupName].forEach((p) => {
      const item = document.createElement("div");
      const isActive = repoLoaded && p.path === (el("currentRepoPath")?.textContent || "");
      item.className = "repo-item" + (isActive ? " active" : "");
      
      // Determine icon based on path or random for demo
      const isPrivate = Math.random() > 0.5; // Replace with actual logic
      const iconSvg = isPrivate ? LockIcon : GlobeIcon;
      
      // Status indicator
      let statusHtml = "";
      const hasUnpushed = Math.random() > 0.7; // Replace with actual logic
      const needsPull = Math.random() > 0.8; // Replace with actual logic
      
      if (needsPull) {
        statusHtml = `<span class="repo-status-icon repo-status-pull">${DownloadIcon}</span>`;
      } else if (hasUnpushed) {
        statusHtml = `<span class="repo-status-dot"></span>`;
      }

      item.innerHTML = `
        <span class="repo-item-icon">${iconSvg}</span>
        <div class="repo-item-info">
          <div class="repo-item-name">${esc(p.name)}</div>
          <div class="repo-item-path">${esc(p.path)}</div>
        </div>
        ${statusHtml}`;

      item.onclick = async () => {
        loadRepo(p.path);
      };

      group.appendChild(item);
    });

    list.appendChild(group);
  });
}

// ── Filter repos ────────────────────────────────────────────────────────

el("repoFilter").oninput = () => {
  const filter = el("repoFilter").value.toLowerCase();
  const items = document.querySelectorAll(".repo-item");
  const groups = document.querySelectorAll(".repo-group");
  
  items.forEach((item) => {
    const name = item.querySelector(".repo-item-name").textContent.toLowerCase();
    const path = item.querySelector(".repo-item-path").textContent.toLowerCase();
    const match = name.includes(filter) || path.includes(filter);
    item.style.display = match ? "flex" : "none";
  });
  
  // Hide empty groups
  groups.forEach((group) => {
    const visible = group.querySelectorAll('.repo-item[style*="display: flex"]').length;
    group.style.display = visible > 0 ? "block" : "none";
  });
};

// ── Mode switching ──────────────────────────────────────────────────────

el("modeSelect").onchange = () => {
  const isBaseline = getMode() === "baseline";
  const isRange = getMode() === "range";
  el("rangePicker").style.display = isRange ? "block" : "none";
  el("baselineBtn").style.display = isBaseline ? "block" : "none";
  el("baselineStatus").style.display = isBaseline ? "block" : "none";
  if (isRange) loadCommits();
  if (isBaseline) updateBaselineStatus();
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
    logLine("Load a repo first — click one from the recent list or use Add to add.");
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
    el("changesEmpty").style.display = "block";
  }
};

// ── Baseline (no-git mode) ──────────────────────────────────────────────

el("baselineBtn").onclick = async () => {
  try {
    await window.go.main.App.SaveBaseline();
    updateBaselineStatus();
    logLine("Baseline saved — make changes, then scan.");
  } catch (err) {
    logLine(`Error saving baseline: ${err}`);
  }
};

async function updateBaselineStatus() {
  const status = el("baselineStatus");
  try {
    const has = await window.go.main.App.HasBaseline();
    status.textContent = has ? "✓ Baseline set — ready to scan" : "No baseline yet — click Set baseline first";
    status.className = has ? "hint ok" : "hint err";
  } catch (_) {
    status.textContent = "Error checking baseline";
    status.className = "hint err";
  }
}

// ── Render file list (sidebar) ──────────────────────────────────────────

function renderFileList() {
  const area = el("fileArea");
  const list = el("fileList");
  const count = el("fileCount");
  const empty = el("changesEmpty");

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
    row.title = f.path; // hover tooltip for full path

    // Extract filename for display
    const parts = f.path.split(/[\\/]/);
    const fileName = parts[parts.length - 1];
    const dirPath = parts.length > 1 ? parts.slice(0, -1).join("/") : "";

    row.innerHTML = `
      <input type="checkbox" class="file-check" data-idx="${i}" checked />
      <span class="file-status status-${f.status}">${f.status}</span>
      <span class="file-path">
        <span class="file-name">${esc(fileName)}</span>
        ${dirPath ? `<span class="file-dir">${esc(dirPath)}</span>` : ""}
      </span>`;

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

// ── Theme toggle ─────────────────────────────────────────────────────────

const MoonIcon = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M21.5 14.0784C20.3003 14.7189 18.9301 15.0821 17.4751 15.0821C12.7491 15.0821 8.91792 11.2509 8.91792 6.52485C8.91792 5.06986 9.28105 3.69968 9.92163 2.5C5.66765 3.49698 2.5 7.31513 2.5 11.8731C2.5 17.1899 6.8101 21.5 12.1269 21.5C16.6849 21.5 20.503 18.3324 21.5 14.0784Z" stroke="currentColor" stroke-width="1.5" fill="none"/></svg>`;
const SunIcon = `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M12 3V4M12 20V21M4 12H3M21 12H20M6.31412 6.31412L5.5 5.5M18.5 18.5L17.6859 17.6859M6.31412 17.6859L5.5 18.5M18.5 5.5L17.6859 6.31412M16 12C16 14.2091 14.2091 16 12 16C9.79086 16 8 14.2091 8 12C8 9.79086 9.79086 8 12 8C14.2091 8 16 9.79086 16 12Z" stroke="currentColor" stroke-width="1.5" fill="none"/></svg>`;

function applyTheme(theme) {
  document.documentElement.setAttribute("data-theme", theme);
  el("themeIcon").innerHTML = theme === "dark" ? SunIcon : MoonIcon;
  localStorage.setItem("theme", theme);
}

el("themeToggle").onclick = () => {
  const current = document.documentElement.getAttribute("data-theme");
  applyTheme(current === "dark" ? "light" : "dark");
};

// ── Resizable changes panel ─────────────────────────────────────────────

(function initResizeHandle() {
  const handle = el("resizeHandle");
  const panel = el("changesPanel");
  if (!handle || !panel) return;

  let dragging = false;
  let startX = 0;
  let startWidth = 0;
  const MIN_WIDTH = 220;
  const MAX_WIDTH = 600;

  handle.addEventListener("mousedown", (e) => {
    dragging = true;
    startX = e.clientX;
    startWidth = panel.offsetWidth;
    handle.classList.add("active");
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    e.preventDefault();
  });

  document.addEventListener("mousemove", (e) => {
    if (!dragging) return;
    const delta = e.clientX - startX;
    const newWidth = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth + delta));
    panel.style.width = newWidth + "px";
  });

  document.addEventListener("mouseup", () => {
    if (!dragging) return;
    dragging = false;
    handle.classList.remove("active");
    document.body.style.cursor = "";
    document.body.style.userSelect = "";
    // Persist width
    try { localStorage.setItem("changesPanelWidth", panel.style.width); } catch (_) {}
  });

  // Restore saved width
  const saved = localStorage.getItem("changesPanelWidth");
  if (saved) panel.style.width = saved;
})();

// ── Init ────────────────────────────────────────────────────────────────

(async () => {
  // Load saved theme or default to light
  const saved = localStorage.getItem("theme") || "light";
  applyTheme(saved);

  const ok = await window.go.main.App.GitAvailable();
  const gitPill = el("gitStatus");
  if (ok) {
    gitPill.textContent = "git ready";
    gitPill.className = "pill pill-ok";
  } else {
    gitPill.textContent = "git not found";
    gitPill.className = "pill pill-err";
    logLine("git not found on PATH — app won't work without git.");
    el("scanBtn").disabled = true;
  }
  loadRecentPaths();
})();