(() => {
  const filters = document.querySelector('.filters');
  if (!filters) return;
  filters.addEventListener('change', () => {
    const allowedSev = new Set(Array.from(filters.querySelectorAll('input[data-sev]:checked')).map(i => i.dataset.sev));
    const allowedCat = new Set(Array.from(filters.querySelectorAll('input[data-cat]:checked')).map(i => i.dataset.cat));
    document.querySelectorAll('#findings details').forEach(d => {
      const sev = (d.className.match(/sev-(\w+)/) || [, ''])[1];
      const cat = d.dataset.category || '';
      d.style.display = (allowedSev.has(sev) && allowedCat.has(cat)) ? '' : 'none';
    });
  });
})();
