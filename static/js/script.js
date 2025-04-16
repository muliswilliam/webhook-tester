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

function sseRequestStream(webhookID) {
    return {
        connect() {
            const source = new EventSource(`/api/webhooks/${webhookID}/stream`);
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
            };
        }
    }
}