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

function serializeHeaderRows(form) {
  const headers = {};
  form.querySelectorAll(".header-row").forEach((row) => {
    const [keyInput, valueInput] = row.querySelectorAll("input");
    const key = keyInput.value.trim();
    const value = valueInput.value.trim();
    if (key && value) headers[key] = value;
  });
  form.querySelector('input[name="response_headers"]').value = JSON.stringify(headers);
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
