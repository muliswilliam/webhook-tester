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
    colorScheme.addEventListener("change", event => {
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
