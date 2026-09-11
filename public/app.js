const state = {
  allNodes: [],
  selectedNodes: new Set(),
  nodeOutputs: {},
  nodeStatus: {},
  activeNode: null,
  pollingTimer: null,
};

const elements = {
  authBackdrop: document.getElementById("authBackdrop"),
  authForm: document.getElementById("authForm"),
  tokenInput: document.getElementById("tokenInput"),
  toggleToken: document.getElementById("toggleToken"),
  authMessage: document.getElementById("authMessage"),
  authSubmit: document.getElementById("authSubmit"),
  appShell: document.getElementById("appShell"),
  sidebar: document.getElementById("sidebar"),
  sidebarScrim: document.getElementById("sidebarScrim"),
  mobileClose: document.getElementById("mobileClose"),
  menuButton: document.getElementById("menuButton"),
  refreshButton: document.getElementById("refreshButton"),
  connectionPill: document.getElementById("connectionPill"),
  connectionText: document.getElementById("connectionText"),
  connectedCount: document.getElementById("connectedCount"),
  connectionSubtext: document.getElementById("connectionSubtext"),
  selectedCount: document.getElementById("selectedCount"),
  consoleStatus: document.getElementById("consoleStatus"),
  statusSubtext: document.getElementById("statusSubtext"),
  targetSummary: document.getElementById("targetSummary"),
  selectedSummary: document.getElementById("selectedSummary"),
  selectedChips: document.getElementById("selectedChips"),
  openNodesMain: document.getElementById("openNodesMain"),
  closeModal: document.getElementById("closeNodes"),
  nodeModal: document.getElementById("nodeModal"),
  nodeList: document.getElementById("nodeList"),
  selectAll: document.getElementById("selectAll"),
  modalCount: document.getElementById("modalCount"),
  taskForm: document.getElementById("taskForm"),
  payloadType: document.getElementById("payloadType"),
  timeoutInput: document.getElementById("timeoutInput"),
  scriptBlock: document.getElementById("scriptBlock"),
  executeButton: document.getElementById("executeButton"),
  terminalOutput: document.getElementById("terminalOutput"),
  outputTitle: document.getElementById("outputTitle"),
  executionTime: document.getElementById("executionTime"),
  terminalStatus: document.getElementById("terminalStatus"),
  outputTabs: document.getElementById("outputTabs"),
  clearOutput: document.getElementById("clearOutput"),
  downloadLink: document.getElementById("downloadLink"),
  logoutButton: document.getElementById("logoutButton"),
  toastRegion: document.getElementById("toastRegion"),
};

function getCookie(name) {
  return document.cookie.split("; ").find((item) => item.startsWith(`${name}=`))?.split("=").slice(1).join("=") || "";
}

function showToast(message, type = "") {
  const toast = document.createElement("div");
  toast.className = `toast ${type}`.trim();
  toast.textContent = message;
  elements.toastRegion.appendChild(toast);
  window.setTimeout(() => toast.remove(), 3600);
}

function setBusy(button, busy, label) {
  if (!button) return;
  if (busy) {
    if (!button.dataset.original) button.dataset.original = button.innerHTML;
    button.disabled = true;
    button.innerHTML = `<span class="spinner-inline"></span>${label}`;
    return;
  }
  button.disabled = false;
  button.innerHTML = button.dataset.original || button.innerHTML;
  button.dataset.original = "";
}

function setConnection(status, text) {
  elements.connectionPill.classList.toggle("online", status === "online");
  elements.connectionText.textContent = text;
  elements.consoleStatus.textContent = status === "online" ? "Operational" : "Offline";
  elements.statusSubtext.textContent = status === "online" ? "management API available" : "management API unavailable";
}

function showAuth(show = true) {
  elements.authBackdrop.hidden = !show;
  elements.authBackdrop.setAttribute("aria-hidden", String(!show));
  elements.appShell.hidden = show;
  if (show) {
    window.setTimeout(() => elements.tokenInput.focus(), 80);
  } else {
    stopNodePolling();
  }
}

function showApp() {
  showAuth(false);
  refreshAll();
}

async function authenticate(token) {
  elements.authMessage.textContent = "";
  setBusy(elements.authSubmit, true, "Authenticating...");
  try {
    await request("/validate_token", { method: "POST", body: JSON.stringify({ token, tool: "RascalRAT", remote_url: window.location.origin }) });
    showApp();
  } catch (error) {
    elements.authMessage.textContent = error.message;
    setBusy(elements.authSubmit, false, "Authenticate");
  }
}

function getQueryToken() {
  const params = new URLSearchParams(window.location.search);
  return params.get("token") || "";
}

async function request(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    ...options,
    headers: {
      ...(options.body ? { "Content-Type": "application/json" } : {}),
      ...options.headers,
    },
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(data.error || `Request failed with status ${response.status}`);
    error.status = response.status;
    throw error;
  }
  return data;
}

function nodeLabel(node) {
  return node.id || String(node);
}

function formatConnected(value) {
  if (!value) return "just now";
  const time = new Date(value);
  if (Number.isNaN(time.getTime())) return "connected";
  const seconds = Math.max(0, Math.round((Date.now() - time.getTime()) / 1000));
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function normalizeNodes(data) {
  return Array.isArray(data) ? data : [];
}

function createSelectIcon() {
  const icon = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  icon.setAttribute("viewBox", "0 0 24 24");
  icon.setAttribute("aria-hidden", "true");
  const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
  path.setAttribute("d", "m7 10 5 5 5-5");
  icon.appendChild(path);
  return icon;
}

function updateSelectionUI() {
  const selected = Array.from(state.selectedNodes);
  elements.selectedCount.textContent = String(selected.length);
  elements.selectedSummary.textContent = selected.length ? `${selected.length} selected: ${selected.join(", ")}` : "No targets selected";
  elements.targetSummary.replaceChildren();
  const summaryText = document.createElement("span");
  summaryText.textContent = selected.length ? `${selected.length} selected: ${selected.join(", ")}` : "No targets selected";
  elements.targetSummary.append(summaryText, createSelectIcon());
  elements.selectedChips.replaceChildren();
  selected.forEach((id) => {
    const chip = document.createElement("span");
    chip.className = "chip";
    const text = document.createElement("span");
    text.textContent = id;
    const remove = document.createElement("button");
    remove.type = "button";
    remove.setAttribute("aria-label", `Remove ${id}`);
    remove.textContent = "×";
    remove.addEventListener("click", () => {
      state.selectedNodes.delete(id);
      updateSelectionUI();
      renderNodeList();
    });
    chip.append(text, remove);
    elements.selectedChips.appendChild(chip);
  });
  elements.selectedChips.classList.toggle("has-items", selected.length > 0);
  renderOutputTabs();
}

function renderNodeList() {
  elements.nodeList.replaceChildren();
  if (state.allNodes.length === 0) {
    const empty = document.createElement("div");
    empty.className = "node-empty";
    empty.textContent = "No clients connected.";
    elements.nodeList.appendChild(empty);
    elements.modalCount.textContent = "0 selected";
    elements.selectAll.checked = false;
    return;
  }
  state.allNodes.forEach((node) => {
    const id = nodeLabel(node);
    const item = document.createElement("label");
    item.className = "node-item";
    const checkbox = document.createElement("input");
    checkbox.type = "checkbox";
    checkbox.value = id;
    checkbox.checked = state.selectedNodes.has(id);
    checkbox.addEventListener("change", () => {
      if (checkbox.checked) state.selectedNodes.add(id);
      else state.selectedNodes.delete(id);
      updateSelectionUI();
    });
    const details = document.createElement("span");
    const title = document.createElement("strong");
    title.textContent = id;
    const connected = document.createElement("small");
    connected.textContent = `Online · ${formatConnected(node.connected)}`;
    details.append(title, connected);
    item.append(checkbox, details);
    elements.nodeList.appendChild(item);
  });
  const selected = Array.from(state.selectedNodes);
  elements.modalCount.textContent = `${selected.length} selected`;
  elements.selectAll.checked = selected.length > 0 && selected.length === state.allNodes.length;
  elements.selectAll.indeterminate = selected.length > 0 && selected.length < state.allNodes.length;
}

function startNodePolling() {
  if (state.pollingTimer) return;
  state.pollingTimer = window.setInterval(fetchNodes, 3000);
}

function stopNodePolling() {
  if (state.pollingTimer) {
    window.clearInterval(state.pollingTimer);
    state.pollingTimer = null;
  }
}

async function fetchNodes() {
  try {
    const data = await request("/nodes");
    const live = normalizeNodes(data).map(nodeLabel);
    state.allNodes = normalizeNodes(data);
    Array.from(state.selectedNodes).forEach((id) => {
      if (!live.includes(id)) state.selectedNodes.delete(id);
    });
    renderNodeList();
    updateSelectionUI();
    setConnection("online", `${state.allNodes.length} client${state.allNodes.length === 1 ? "" : "s"} online`);
  } catch (error) {
    if (error.status === 401) {
      showAuth(true);
      return;
    }
    setConnection("offline", "clients unavailable");
    showToast(error.message, "error");
  }
}

function openNodeModal() {
  elements.nodeModal.hidden = false;
  fetchNodes();
  startNodePolling();
}

function closeNodeModal() {
  elements.nodeModal.hidden = true;
  stopNodePolling();
}

function renderOutputTabs() {
  const tabs = elements.outputTabs;
  tabs.replaceChildren();
  const selected = Array.from(state.selectedNodes);
  if (selected.length === 0) {
    const empty = document.createElement("span");
    empty.className = "tab-empty";
    empty.textContent = "No targets selected";
    tabs.appendChild(empty);
    if (state.activeNode && !state.selectedNodes.has(state.activeNode)) state.activeNode = null;
    showActiveOutput();
    return;
  }
  if (!state.activeNode || !state.selectedNodes.has(state.activeNode)) state.activeNode = selected[0];
  selected.forEach((id) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `tab-btn${id === state.activeNode ? " active" : ""}`;
    const dot = document.createElement("span");
    dot.className = `tab-dot${state.nodeStatus[id] ? ` ${state.nodeStatus[id]}` : ""}`;
    const text = document.createElement("span");
    text.textContent = id;
    button.append(dot, text);
    button.addEventListener("click", () => {
      state.activeNode = id;
      renderOutputTabs();
    });
    tabs.appendChild(button);
  });
  showActiveOutput();
}

function showActiveOutput() {
  const output = state.activeNode ? state.nodeOutputs[state.activeNode] : null;
  elements.outputTitle.textContent = state.activeNode ? `TERMINAL OUTPUT :: ${state.activeNode}` : "TERMINAL OUTPUT";
  if (output) {
    elements.terminalOutput.textContent = output.text;
    elements.terminalOutput.style.color = output.color;
  } else {
    elements.terminalOutput.textContent = state.activeNode ? `No output captured for ${state.activeNode} yet.` : "Select target nodes and run a command to view output.";
    elements.terminalOutput.style.color = "";
  }
}

function clearOutput() {
  state.nodeOutputs = {};
  state.nodeStatus = {};
  elements.terminalOutput.textContent = "Output cleared.";
  elements.terminalOutput.style.color = "";
  elements.executionTime.textContent = "Duration: --";
  elements.terminalStatus.textContent = "No command running";
  renderOutputTabs();
}

async function submitTask(event) {
  event.preventDefault();
  const selected = Array.from(state.selectedNodes);
  if (selected.length === 0) {
    showToast("Select at least one target node.", "error");
    openNodeModal();
    return;
  }
  const script = elements.scriptBlock.value.trim();
  if (!script) {
    elements.scriptBlock.focus();
    showToast("Enter a command before executing.", "error");
    return;
  }
  const timeoutSeconds = Math.max(1, Math.min(600, Number(elements.timeoutInput.value) || 10));
  elements.executeButton.disabled = true;
  elements.executeButton.innerHTML = `<span class="spinner-inline"></span>Executing on ${selected.length} node${selected.length === 1 ? "" : "s"}...`;
  elements.terminalStatus.textContent = `Dispatching to ${selected.length} node${selected.length === 1 ? "" : "s"}...`;
  elements.executionTime.textContent = "Duration: --";
  state.nodeOutputs = {};
  state.nodeStatus = {};
  state.activeNode = selected[0];
  elements.terminalOutput.textContent = `[+] Packaging task for ${selected.length} node${selected.length === 1 ? "" : "s"}...\n[+] Dispatching instruction packet to: ${selected.join(", ")}`;
  elements.terminalOutput.style.color = "";
  renderOutputTabs();
  try {
    const data = await request("/nodes/task", {
      method: "POST",
      body: JSON.stringify({
        node_ids: selected,
        payload_type: elements.payloadType.value,
        script_block: script,
        timeout: timeoutSeconds * 1_000_000_000,
      }),
    });
    const responses = data.responses || {};
    selected.forEach((id) => {
      const response = responses[id];
      if (response?.success) {
        state.nodeOutputs[id] = { text: response.stdout || "[✓] Script executed successfully with empty standard return streams.", color: "#b9e8d5" };
        state.nodeStatus[id] = "ok";
      } else if (response) {
        state.nodeOutputs[id] = {
          text: `[!] Target Application Error:\n${response.error_message || ""}\n\n[!] Standard Error Output:\n${response.stderr || ""}\n\n[!] Partial Output Stream:\n${response.stdout || ""}`,
          color: "#ff9aa4",
        };
        state.nodeStatus[id] = "err";
      } else {
        state.nodeOutputs[id] = { text: "[!] No response received from node.", color: "#ff9aa4" };
        state.nodeStatus[id] = "err";
      }
    });
    elements.terminalStatus.textContent = `Completed in ${(timeoutSeconds * 1000).toFixed(0)}ms requested window`;
    elements.executionTime.textContent = "Duration: --";
    showToast("Command dispatch completed.", "success");
  } catch (error) {
    elements.terminalOutput.textContent = `[CRITICAL SERVER FAULT]\nStatus: ${error.status || "unknown"}\nError: ${error.message}`;
    elements.terminalOutput.style.color = "#ff9aa4";
    elements.terminalStatus.textContent = "Dispatch failed";
    showToast(error.message, "error");
  } finally {
    elements.executeButton.disabled = false;
    elements.executeButton.innerHTML = `<span>Execute on selected clients</span><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h13M13 6l6 6-6 6"/></svg>`;
    renderOutputTabs();
  }
}

async function refreshAll() {
  setConnection("checking", "Checking connection");
  try {
    await request("/status");
    setConnection("online", "console ready");
  } catch (error) {
    if (error.status === 401) {
      showAuth(true);
      return;
    }
    setConnection("offline", "console offline");
  }
  fetchNodes();
}

function initEvents() {
  elements.authForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const token = elements.tokenInput.value.trim();
    if (!token) return;
    await authenticate(token);
  });
  elements.toggleToken.addEventListener("click", () => {
    const showing = elements.tokenInput.type === "text";
    elements.tokenInput.type = showing ? "password" : "text";
    elements.toggleToken.textContent = showing ? "Show" : "Hide";
    elements.toggleToken.setAttribute("aria-label", showing ? "Show token" : "Hide token");
  });
  elements.logoutButton.addEventListener("click", async () => {
    try { await request("/logout", { method: "POST" }); } catch (_) { /* local cleanup still applies */ }
    state.allNodes = [];
    state.selectedNodes.clear();
    state.nodeOutputs = {};
    state.nodeStatus = {};
    updateSelectionUI();
    showAuth(true);
  });
  elements.refreshButton.addEventListener("click", () => refreshAll());
  elements.openNodesMain.addEventListener("click", openNodeModal);
  elements.closeModal.addEventListener("click", closeNodeModal);
  elements.nodeModal.addEventListener("click", (event) => { if (event.target === elements.nodeModal) closeNodeModal(); });
  elements.selectAll.addEventListener("change", (event) => {
    if (event.target.checked) state.allNodes.forEach((node) => state.selectedNodes.add(nodeLabel(node)));
    else state.selectedNodes.clear();
    renderNodeList();
    updateSelectionUI();
  });
  elements.taskForm.addEventListener("submit", submitTask);
  elements.clearOutput.addEventListener("click", clearOutput);
  elements.menuButton.addEventListener("click", () => {
    elements.sidebar.classList.add("open");
    elements.sidebarScrim.classList.add("open");
  });
  elements.mobileClose.addEventListener("click", () => {
    elements.sidebar.classList.remove("open");
    elements.sidebarScrim.classList.remove("open");
  });
  elements.sidebarScrim.addEventListener("click", () => {
    elements.sidebar.classList.remove("open");
    elements.sidebarScrim.classList.remove("open");
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !elements.nodeModal.hidden) closeNodeModal();
  });
}

function init() {
  initEvents();
  const urlToken = getQueryToken();
  if (urlToken) {
    authenticate(urlToken);
    return;
  }
  if (getCookie("rascalrat_token")) showApp();
  else showAuth(true);
}

init();
