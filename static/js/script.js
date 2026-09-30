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
  const container = button.closest(".space-y-2").querySelector(".header-rows");
  container.appendChild(template.content.cloneNode(true));
}

function removeHeaderRow(button) {
  button.closest(".header-row").remove();
}

// Serializes the header rows into the hidden response_headers field as a
// {name: value} object. Blank rows are skipped; a value without a name blocks
// submission. Returns whether the form may submit.
function serializeHeaderRows(form) {
  const headers = {};
  for (const row of form.querySelectorAll(".header-row")) {
    const [nameInput, valueInput] = row.querySelectorAll("input");
    const name = nameInput.value.trim();
    const value = valueInput.value.trim();
    nameInput.setCustomValidity(!name && value ? "Enter a header name, or remove this row." : "");
    if (!nameInput.reportValidity()) return false;
    if (name) headers[name] = value;
  }
  form.querySelector('input[name="response_headers"]').value = JSON.stringify(headers);
  return true;
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
