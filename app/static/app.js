(() => {
  document.querySelectorAll("[data-toggle-password]").forEach((button) => {
    button.addEventListener("click", () => {
      const input = document.getElementById(button.getAttribute("data-toggle-password"));
      if (!input) return;
      const showing = input.type === "password";
      input.type = showing ? "text" : "password";
      button.textContent = showing ? "隐藏" : "显示";
      button.setAttribute("aria-pressed", showing ? "true" : "false");
      input.focus();
    });
  });

  document.querySelectorAll("form[data-auth]").forEach((form) => {
    const password = form.querySelector("[name=password]");
    const confirm = form.querySelector("[name=confirm]");
    const slot = form.querySelector("#confirm-error");

    const syncConfirm = () => {
      if (!password || !confirm || !slot) return;
      if (confirm.value === "" || password.value === confirm.value) {
        slot.hidden = true;
      }
    };
    password?.addEventListener("input", syncConfirm);
    confirm?.addEventListener("input", syncConfirm);

    form.addEventListener("submit", (event) => {
      if (password && confirm && confirm.value !== password.value) {
        event.preventDefault();
        if (slot) {
          slot.hidden = false;
          slot.textContent = "两次密码不一致";
        }
        confirm.setAttribute("aria-invalid", "true");
        confirm.focus();
        return;
      }
      const button = form.querySelector("[type=submit]");
      if (button) {
        button.disabled = true;
        button.textContent = "请稍候…";
      }
    });
  });
})();
