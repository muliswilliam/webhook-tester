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

function currentTheme() {
    return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function updateThemeToggle() {
    const toggle = document.getElementById("theme-toggle");
    if (!toggle) return;

    const nextTheme = currentTheme() === "dark" ? "light" : "dark";
    const label = `Switch to ${nextTheme} theme`;
    toggle.setAttribute("aria-label", label);
    toggle.setAttribute("title", label);
}

function applyTheme(theme, persist = true) {
    const useDark = theme === "dark";
    document.documentElement.classList.toggle("dark", useDark);
    document.documentElement.dataset.mode = useDark ? "dark" : "light";
    document.documentElement.style.colorScheme = useDark ? "dark" : "light";
    if (persist) localStorage.setItem("webhook-tester-theme", theme);
    updateThemeToggle();
}

function toggleTheme() {
    applyTheme(currentTheme() === "dark" ? "light" : "dark");
}

document.addEventListener("DOMContentLoaded", () => {
    updateThemeToggle();

    const colorScheme = window.matchMedia("(prefers-color-scheme: dark)");
    colorScheme.addEventListener("change", event => {
        if (!localStorage.getItem("webhook-tester-theme")) {
            applyTheme(event.matches ? "dark" : "light", false);
        }
    });
});

function sseRequestStream(webhookID) {
    return {
        connect() {
            const source = new EventSource(`/webhook-stream/${webhookID}`);
            source.onmessage = e => {
                const req = JSON.parse(e.data);
                const wrapper = document.createElement("a");
                wrapper.className = "flex flex-row gap-2 p-2 rounded border hover:bg-blue-50 hover:border-blue-200 cursor-pointer"
                wrapper.href = `/?requests/${req.id}?address=${req.webhook_id}`
                wrapper.innerHTML = `
                        <div class="text-xs text-gray-600 font-medium">${req.method}</div>
                        <div class="text-blue-600 font-mono text-xs break-all">${req.id}</div>
                    `;

                const container = document.getElementById(`request-log-${webhookID}`);
                container.insertBefore(wrapper, container.firstChild);
                location.reload()
            };
        }
    }
}
