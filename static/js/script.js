function copyCurl() {
    return {
        copied: false,
        copy(el) {
            navigator.clipboard.writeText(el.innerText);
            this.copied = true;
            setTimeout(() => this.copied = false, 1500);
        }
    }
}

function themePreference() {
    const saved = localStorage.getItem("webhook-tester-theme");
    return ["system", "light", "dark"].includes(saved) ? saved : "system";
}

function resolvedTheme(preference) {
    if (preference === "system") {
        return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    }
    return preference;
}

function applyTheme(preference, persist = true) {
    const theme = resolvedTheme(preference);
    const useDark = theme === "dark";
    document.documentElement.classList.toggle("dark", useDark);
    document.documentElement.dataset.mode = theme;
    document.documentElement.dataset.theme = preference;
    document.documentElement.style.colorScheme = theme;
    if (persist) localStorage.setItem("webhook-tester-theme", preference);

    const select = document.getElementById("theme-select");
    if (select) select.value = preference;
}

function setThemePreference(preference) {
    applyTheme(preference);
}

document.addEventListener("DOMContentLoaded", () => {
    applyTheme(themePreference(), false);

    const colorScheme = window.matchMedia("(prefers-color-scheme: dark)");
    colorScheme.addEventListener("change", () => {
        if (themePreference() === "system") {
            applyTheme("system", false);
        }
    });
});

function sseRequestStream(webhookID) {
    return {
        connect() {
            const source = new EventSource(`/webhook-stream/${webhookID}`);
            source.onmessage = e => {
                const req = JSON.parse(e.data);
                prependSidebarRequest(webhookID, req);

                const activeWebhookID = document.querySelector(".app-shell")?.dataset.activeWebhook;
                if (activeWebhookID && activeWebhookID === webhookID) {
                    prependMainRequest(webhookID, req);
                }
            };
        }
    }
}

// Builds DOM nodes via textContent (never innerHTML) since method/id/body/headers
// originate from arbitrary, untrusted webhook payloads sent over the network.
function prependSidebarRequest(webhookID, req) {
    const container = document.getElementById(`request-log-${webhookID}`);
    if (!container) return;

    const placeholder = container.querySelector(":scope > p");
    if (placeholder) placeholder.remove();

    const link = document.createElement("a");
    link.className = "sidebar-request";
    link.href = `/requests/${encodeURIComponent(req.id)}?address=${encodeURIComponent(req.webhook_id)}`;

    const methodBadge = document.createElement("span");
    methodBadge.className = "method-badge";
    methodBadge.textContent = req.method;

    const idLabel = document.createElement("span");
    idLabel.className = "truncate font-mono text-[11px] text-muted";
    idLabel.textContent = req.id;

    link.append(methodBadge, idLabel);
    container.insertBefore(link, container.firstChild);
}

function prependMainRequest(webhookID, req) {
    const list = document.getElementById(`request-log-list-${webhookID}`);
    if (!list) return;

    const empty = document.getElementById(`request-log-empty-${webhookID}`);
    if (empty) empty.remove();

    const card = document.createElement("a");
    card.className = "surface overflow-hidden request-row";
    card.href = `/requests/${encodeURIComponent(req.id)}?address=${encodeURIComponent(req.webhook_id)}`;
    card.dataset.requestId = req.id;

    const left = document.createElement("span");
    left.className = "flex min-w-0 items-center gap-3";
    const methodBadge = document.createElement("span");
    methodBadge.className = "method-badge";
    methodBadge.textContent = req.method;
    const idLabel = document.createElement("span");
    idLabel.className = "truncate font-mono text-xs";
    idLabel.textContent = req.id;
    left.append(methodBadge, idLabel);

    const right = document.createElement("span");
    right.className = "flex shrink-0 items-center gap-3";
    const timeLabel = document.createElement("span");
    timeLabel.className = "hidden text-xs text-muted sm:block";
    timeLabel.textContent = formatReceivedAt(req.received_at);
    const newBadge = document.createElement("span");
    newBadge.className = "new-badge";
    newBadge.textContent = "New";
    right.append(timeLabel, newBadge);

    card.append(left, right);
    list.insertBefore(card, list.firstChild);

    const counter = document.getElementById(`request-count-${webhookID}`);
    if (counter) {
        const next = (parseInt(counter.dataset.count || "0", 10) || 0) + 1;
        counter.dataset.count = String(next);
        counter.textContent = `${next} captured ${next === 1 ? "request" : "requests"}`;
    }
}

function formatReceivedAt(iso) {
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return "";
    return date.toISOString().replace("T", " ").replace(/\.\d+Z$/, " UTC");
}
