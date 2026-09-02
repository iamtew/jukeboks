document.addEventListener('DOMContentLoaded', async () => {
  const target = document.getElementById('readme');
  if (!target) return;

  try {
    const response = await fetch('./README.md');
    if (!response.ok) {
      throw new Error(response.statusText);
    }
    const markdown = await response.text();
    target.innerHTML = marked.parse(markdown);
  } catch (err) {
    target.textContent = 'Could not load README.md';
  }
});
