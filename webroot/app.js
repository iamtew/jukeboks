document.addEventListener('DOMContentLoaded', () => {
  const buttons = document.querySelectorAll('[data-copy]');

  buttons.forEach((button) => {
    button.addEventListener('click', async () => {
      const value = button.getAttribute('data-copy');
      if (!value) return;

      try {
        await navigator.clipboard.writeText(window.location.origin + value);
        const original = button.textContent;
        button.textContent = 'Copied!';
        window.setTimeout(() => {
          button.textContent = original;
        }, 1200);
      } catch (err) {
        button.textContent = 'Copy failed';
      }
    });
  });
});
