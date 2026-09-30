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

function addHeaderRow(button) {
  const template = document.getElementById("header-row-template");
  const container = button.closest("[data-header-editor]").querySelector(".header-rows");
  container.appendChild(template.content.cloneNode(true));
  container.lastElementChild.querySelector("input").focus();
}

function removeHeaderRow(button) {
  button.closest(".header-row").remove();
}

// Header field names are RFC 9110 tokens. Mirrors models.ValidateResponseHeaders.
const HEADER_NAME = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/;
const SERVER_MANAGED_HEADERS = ["content-length", "transfer-encoding"];

function headerNameError(name, value) {
  if (!name) return value ? "Enter a header name, or remove this row." : "";
  if (!HEADER_NAME.test(name)) return "Header names can't contain spaces or special characters.";
  if (SERVER_MANAGED_HEADERS.includes(name.toLowerCase())) {
    return `${name} is set by the server and can't be overridden.`;
  }
  return "";
}

// Serializes the header rows into the hidden response_headers field as a
// {name: value} object. Blank rows are skipped; an invalid name blocks
// submission. Returns whether the form may submit.
function serializeHeaderRows(form) {
  const headers = {};
  for (const row of form.querySelectorAll(".header-row")) {
    const [nameInput, valueInput] = row.querySelectorAll("input");
    const name = nameInput.value.trim();
    const value = valueInput.value.trim();
    nameInput.setCustomValidity(headerNameError(name, value));
    if (!nameInput.reportValidity()) return false;
    if (name) headers[name] = value;
  }
  form.querySelector('input[name="response_headers"]').value = JSON.stringify(headers);
  return true;
}

// Asks for confirmation in the #confirm-dialog before a destructive form
// submits. Use as onsubmit="return confirmSubmit(event, this)", with the
// dialog's copy in the form's data-confirm-title, data-confirm-message and
// data-confirm-label attributes.
function confirmSubmit(event, form) {
  if (form.dataset.confirmed === "true") {
    delete form.dataset.confirmed;
    return true;
  }
  event.preventDefault();

  const dialog = document.getElementById("confirm-dialog");
  dialog.querySelector("#confirm-dialog-title").textContent = form.dataset.confirmTitle;
  dialog.querySelector("#confirm-dialog-message").textContent = form.dataset.confirmMessage;
  dialog.querySelector("#confirm-dialog-confirm").textContent = form.dataset.confirmLabel;
  // Closing via Escape or the backdrop leaves returnValue as is, so reset it.
  dialog.returnValue = "";
  dialog.addEventListener(
    "close",
    () => {
      if (dialog.returnValue !== "confirm") return;
      form.dataset.confirmed = "true";
      form.requestSubmit();
    },
    { once: true },
  );
  dialog.showModal();
  return false;
}

// Copies text to the clipboard, then flashes setCopied(true) for a moment so
// the triggering button can confirm the copy.
function copyWithFeedback(text, setCopied) {
  navigator.clipboard.writeText(text).then(
    () => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    },
    (err) => console.error("Copy to clipboard failed", err),
  );
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
