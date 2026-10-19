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

const serverForm = document.getElementById('server-form');
if (serverForm) {
  const fa = root.lang === 'fa';
  const errors = document.getElementById('server-errors');
  const field = name => serverForm.elements.namedItem(name);
  const localizeError = message => {
    if (!fa) return message;
    const translations = {
      'name: use 1–27 English letters, numbers, dots, underscores or hyphens; start with a letter or number': 'نام سرور باید ۱ تا ۲۷ نویسه انگلیسی باشد؛ با حرف یا عدد شروع شود و فقط نقطه، خط تیره یا زیرخط داشته باشد.',
      'endpoint: enter a domain or IP address without https://, a port or a path': 'دامنه یا IP را بدون https://، شماره پورت و مسیر وارد کنید.',
      'public_ip: enter a public IP address': 'IP عمومی معتبر وارد کنید؛ IP خصوصی یا آدرس محلی پذیرفته نمی‌شود.',
      'use TCP or UDP on an unprivileged port 1024–65535': 'برای این پروتکل TCP یا UDP و پورتی بین ۱۰۲۴ تا ۶۵۵۳۵ انتخاب کنید.',
      'use a canonical IPv4 /16–/30 VPN subnet': 'زیرشبکه IPv4 معتبر با پیشوند ۱۶ تا ۳۰ وارد کنید؛ نمونه: 10.80.0.0/24.',
      'invalid pool range or gateway outside subnet': 'ابتدا و انتهای محدوده و دروازه باید IPv4 معتبر و داخل زیرشبکه باشند؛ انتهای محدوده نباید قبل از ابتدای آن باشد.',
      'pool must exclude network, broadcast and gateway': 'محدوده IP نباید شامل آدرس شبکه، آدرس broadcast یا دروازه باشد.',
      'provide 1–3 IPv4 DNS servers': 'یک تا سه آدرس IPv4 برای DNS وارد کنید و با ویرگول انگلیسی جدا کنید.',
      'invalid DNS address': 'آدرس DNS نامعتبر است؛ فقط IPv4 وارد کنید.',
      'provide 1–32 explicit allowed IPv4 networks; full tunnel is unavailable': 'یک تا ۳۲ مقصد IPv4 با CIDR وارد کنید؛ اینترنت تمام‌تونل پشتیبانی نمی‌شود.',
      'allowed destinations must be canonical IPv4 CIDRs, distinct from the VPN subnet and not a default route': 'مقصدهای مجاز باید IPv4 همراه CIDR باشند، با زیرشبکه VPN هم‌پوشانی نداشته باشند و مسیر پیش‌فرض نباشند.',
      'VPN subnet overlaps an existing router address or route': 'زیرشبکه VPN با آدرس یا مسیر موجود در روتر هم‌پوشانی دارد؛ زیرشبکه دیگری انتخاب کنید.',
      'VPN subnet overlaps existing address pool': 'زیرشبکه VPN با محدوده IP موجود در روتر هم‌پوشانی دارد.',
      'SSTP is already enabled or its state is unknown; existing service will not be replaced': 'سرویس SSTP روتر از قبل فعال است یا وضعیت آن مشخص نیست؛ ابتدا تنظیمات موجود را بررسی کنید.',
      'existing IPsec peers require manual conflict review before creating IKEv2': 'روتر از قبل peer مربوط به IPsec دارد؛ پیش از ساخت IKEv2، تداخل تنظیمات موجود را بررسی کنید.',
      'permission denied': 'حساب شما مجوز انجام این عملیات روی روتر انتخاب‌شده را ندارد.'
    };
    return translations[message] || message;
  };
  const showError = message => {
    errors.textContent = message;
    errors.hidden = false;
    errors.focus();
  };
  serverForm.addEventListener('submit', async event => {
    event.preventDefault();
    errors.hidden = true;
    const endpoint = field('endpoint').value.trim();
    const publicIP = field('public_ip').value.trim();
    if (!endpoint && !publicIP) {
      showError(fa ? 'دامنه یا IP عمومی سرور را وارد کنید. اگر دامنه ندارید، IP عمومی الزامی است.' : 'Enter a domain or a public server IP. Public IP is required when no domain is supplied.');
      return;
    }
    if (!field('input_anchor').value || !field('forward_anchor').value) {
      showError(fa ? 'ابتدا روتر را انتخاب و دکمه بررسی فایروال را بزنید.' : 'Select a router and inspect its firewall first.');
      return;
    }
    const inspectedRouter = document.querySelector('.firewall-inspector select').value;
    if (field('router_id').value !== inspectedRouter) {
      showError(fa ? 'روتر تغییر کرده است؛ فایروال همین روتر را دوباره بررسی کنید.' : 'The selected router changed. Inspect its firewall again.');
      return;
    }
    const button = serverForm.querySelector('button[type=submit]');
    button.disabled = true;
    try {
      const values = Object.fromEntries(new FormData(serverForm));
      const spec = {...values, endpoint, public_ip: publicIP, port: Number(values.port), dns: values.dns.split(',').map(v => v.trim()).filter(Boolean), allowed: values.allowed.split(',').map(v => v.trim()).filter(Boolean)};
      delete spec.csrf; delete spec.idempotency_key;
      const response = await fetch(serverForm.action, {method: 'POST', headers: {'Content-Type': 'application/json', 'X-CSRF-Token': document.querySelector('meta[name="csrf-token"]').content}, body: JSON.stringify({spec, idempotency_key: values.idempotency_key})});
      const result = await response.json();
      if (!response.ok) {
        showError((fa ? 'ذخیره انجام نشد: ' : 'Unable to save: ') + localizeError(result.error?.message || String(response.status)));
        return;
      }
      location.assign('/?page=job&id=' + encodeURIComponent(result.id));
    } catch (_) {
      showError(fa ? 'ارتباط با پنل برقرار نشد؛ اطلاعات فرم حفظ شده است. دوباره تلاش کنید.' : 'Could not reach the panel. Your form is preserved; retry.');
    } finally { button.disabled = false; }
  });
}
