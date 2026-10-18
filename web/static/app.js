'use strict';
const root = document.documentElement;
try { root.dataset.theme = localStorage.getItem('mvm-theme') || 'dark'; } catch (_) {}
document.querySelector('#theme-toggle')?.addEventListener('click', () => {
  root.dataset.theme = root.dataset.theme === 'light' ? 'dark' : 'light';
  try { localStorage.setItem('mvm-theme', root.dataset.theme); } catch (_) {}
});
document.querySelectorAll('[data-copy]').forEach(button => button.addEventListener('click', async () => {
  const value = document.getElementById(button.dataset.copy)?.value;
  if (value) { try { await navigator.clipboard.writeText(value); document.getElementById('live-status').textContent = root.lang === 'fa' ? 'پیوند کپی شد' : 'Link copied'; } catch (_) {} }
}));
const fragmentToken = document.getElementById('fragment-token');
const loginForm = document.getElementById('login-form');
if (loginForm && new URLSearchParams(location.search).get('page') === 'delivery' && /^[A-Za-z0-9_-]{43}$/.test(location.hash.slice(1))) {
  // Fragments never reach HTTP logs. Redirects inherit this fragment through sign-in.
  loginForm.action = '/login' + location.hash;
  const next = document.createElement('input');
  next.type = 'hidden'; next.name = 'next'; next.value = 'delivery'; loginForm.appendChild(next);
}
if (fragmentToken && /^[A-Za-z0-9_-]{43}$/.test(location.hash.slice(1))) {
  fragmentToken.value = location.hash.slice(1);
  history.replaceState(null, '', location.pathname + location.search);
}
document.body.addEventListener('htmx:configRequest', event => {
  event.detail.headers['X-CSRF-Token'] = document.querySelector('meta[name="csrf-token"]')?.content || '';
});
document.body.addEventListener('htmx:responseError', () => {
  document.getElementById('live-status').textContent = root.lang === 'fa' ? 'عملیات ناموفق بود؛ صفحه را بازخوانی کنید و گزارش را بررسی کنید.' : 'Operation failed. Reload the page and review its status.';
});
