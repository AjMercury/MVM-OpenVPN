(() => {
  const fa = document.documentElement.lang === 'fa';
  const translate = message => {
    if (!fa) return message;
    const known = {
      'router authentication failed; check username, password and api/rest-api policies': 'نام کاربری یا رمز اشتباه است، یا مجوزهای api و rest-api فعال نیستند.',
      'router access denied; check user policies and service address restrictions': 'دسترسی رد شد؛ مجوزهای کاربر و آدرس‌های مجاز www-ssl را بررسی کنید.',
      'REST endpoint not found; check RouterOS 7 and the www-ssl port': 'مسیر REST پیدا نشد؛ نسخه RouterOS و پورت www-ssl را بررسی کنید.',
      'TLS certificate is not trusted; supply the router CA PEM': 'گواهی روتر قابل اعتماد نیست؛ گواهی عمومی CA روتر را در تنظیمات پیشرفته وارد کنید.',
      'TLS certificate does not match the router IP or hostname': 'گواهی برای این IP یا دامنه صادر نشده است؛ آدرس اتصال باید با SAN گواهی مطابقت داشته باشد.',
      'TLS certificate is expired or invalid; check certificates and clocks': 'گواهی منقضی یا نامعتبر است؛ تاریخ گواهی و ساعت روتر و سرور را بررسی کنید.',
      'router DNS resolution failed': 'دامنه روتر پیدا نشد؛ DNS را بررسی کنید یا IP روتر را وارد کنید.',
      'router destination is outside MVM_ROUTER_CIDRS': 'IP روتر در فهرست مجاز سرور پنل نیست؛ آن را به MVM_ROUTER_CIDRS در /etc/mvm/config.env اضافه و سرویس mvm را ری‌استارت کنید.',
      'router destination is forbidden': 'آدرس محلی یا link-local برای اتصال روتر مجاز نیست؛ IP مدیریتی روتر را وارد کنید.',
      'router is unreachable; check www-ssl port, firewall and routing': 'روتر در دسترس نیست؛ www-ssl، شماره پورت، مسیر شبکه و قانون فایروالِ IP سرور پنل را بررسی کنید.',
      'router request timed out; mutation outcome may be uncertain': 'زمان اتصال تمام شد؛ مسیر شبکه، فایروال و پورت روتر را بررسی کنید.',
      'invalid router connection fields': 'IP یا دامنه، پورت، نام کاربری و رمز اتصال را کامل و صحیح وارد کنید.',
      'invalid private CA PEM': 'متن گواهی CA معتبر نیست؛ فایل عمومی با BEGIN CERTIFICATE را وارد کنید.',
      'invalid router inventory fields': 'نام، IP یا دامنه، پورت، نام کاربری و رمز روتر را کامل وارد کنید.',
      'cannot verify other PPP services before sharing credentials': 'مجوز بررسی سرویس‌های PPP کافی نیست؛ دسترسی کاربر روتر را بررسی کنید.',
      'cannot verify PPTP is disabled before sharing credentials': 'وضعیت PPTP مشخص نیست؛ قبل از اشتراک رمز، غیرفعال بودن آن باید قابل بررسی باشد.',
      'disable unmanaged PPTP/PPPoE services before sharing PPP credentials': 'برای حساب PPP مشترک، سرویس‌های PPTP و PPPoE خارج از پنل باید غیرفعال باشند.',
      'cannot verify shared PPP listener scope': 'محدوده سرویس‌های PPP قابل بررسی نیست؛ مجوز کاربر روتر را بررسی کنید.',
      'unmanaged PPP listener must be reviewed before sharing credentials': 'یک سرویس PPP خارج از پنل فعال است؛ قبل از اشتراک رمز آن را بررسی کنید.',
      'shared identity router, username or owner does not match': 'سرویس جدید باید روی همان روتر و با همان نام و مالک کاربر ساخته شود.',
      'username already exists in PPP scope': 'این نام قبلاً در PPP روتر ساخته شده است؛ کاربر دیگری را انتخاب کنید.',
      'shared PPP services changed; review the user access scope': 'سرویس‌های PPP تغییر کرده‌اند؛ دسترسی کاربر مشترک باید دوباره بررسی شود.',
      'this username already exists; add WireGuard from the existing user': 'این کاربر وجود دارد؛ از صفحه کاربران، گزینه افزودن سرویس همان کاربر را انتخاب کنید.',
      'disable shared PPP users before adding another PPP server; its authentication scope must be reviewed': 'قبل از افزودن سرور PPP جدید، کاربران مشترک را غیرفعال و محدوده دسترسی را دوباره بررسی کنید.'
    };
    return known[message] || message;
  };
  document.querySelectorAll('[data-router-error]').forEach(node => { node.textContent = translate(node.textContent.trim()); });
  const send = async (form, url) => {
    const values = Object.fromEntries(new FormData(form));
    delete values.csrf;
    if ('port' in values) values.port = Number(values.port);
    if (form.id === 'user-form') values.shared = values.shared === 'yes';
    const response = await fetch(url, {method: 'POST', headers: {'Content-Type':'application/json','X-CSRF-Token': document.querySelector('meta[name="csrf-token"]').content, Accept: 'application/json'}, body: JSON.stringify(values)});
    const data = await response.json();
    if (!response.ok) throw new Error(translate(data.error?.message || String(response.status)));
    return data;
  };
  const router = document.getElementById('router-form');
  if (router) {
    const result = document.getElementById('router-result');
    const buttons = router.querySelectorAll('button');
    const run = async save => {
      if (!router.reportValidity()) return;
      buttons.forEach(b => { b.disabled = true; });
      result.hidden = false; result.dataset.status = 'testing';
      result.textContent = fa ? 'در حال بررسی اتصال امن، نام کاربری و مجوزها…' : 'Checking secure connection, credentials and permissions…';
      try {
        const data = await send(router, save ? router.action : '/api/v1/routers/test');
        if (save) { location.assign('/?page=router&id=' + encodeURIComponent(data.id)); return; }
        result.dataset.status = data.status;
        const warning = data.observed?.warnings?.length > 0;
        result.textContent = warning ? (fa ? 'اتصال برقرار شد، ولی بعضی مجوزها یا قابلیت‌ها کافی نیست. دسترسی read, api, rest-api و نسخه پایدار RouterOS 7.17+ را بررسی کنید.' : 'Connected, but some permissions or capabilities are missing. Check read, api, rest-api policies and stable RouterOS 7.17+.') : (fa ? 'اتصال برقرار شد؛ روتر آماده ذخیره است.' : 'Connected. The router is ready to save.');
        if (data.observed?.version) result.textContent += ' RouterOS ' + data.observed.version;
      } catch (error) {
        result.dataset.status = 'offline';
        result.textContent = (fa ? 'اتصال برقرار نشد: ' : 'Connection failed: ') + (error.message === 'Failed to fetch' ? (fa ? 'ارتباط با سرور پنل قطع شد؛ دوباره تلاش کنید.' : 'Panel unreachable. Retry.') : error.message);
        if (result.textContent.includes('CA')) router.querySelector('details').open = true;
      } finally { buttons.forEach(b => { b.disabled = false; }); result.focus(); }
    };
    document.getElementById('router-test').addEventListener('click', () => run(false));
    router.addEventListener('submit', event => { event.preventDefault(); run(true); });
    router.addEventListener('input', () => { result.hidden = true; });
  }
  const user = document.getElementById('user-form');
  const savedRouter = document.getElementById('saved-router-form');
  if (savedRouter) {
    savedRouter.addEventListener('submit', async event => {
      event.preventDefault();
      const output = document.getElementById('saved-router-result');
      const button = savedRouter.querySelector('button'); button.disabled = true;
      output.hidden = false; output.textContent = fa ? 'در حال تست اتصال…' : 'Testing connection…';
      try { await send(savedRouter, savedRouter.action); location.reload(); }
      catch (error) { output.textContent = error.message; output.dataset.status = 'offline'; output.focus(); }
      finally { button.disabled = false; }
    });
  }
  if (user) {
    const select = document.getElementById('user-server');
    const update = () => {
      const protocol = select.selectedOptions[0]?.dataset.protocol || '';
      const wg = protocol === 'wireguard';
      document.documentElement.dataset.protocol = protocol;
      document.getElementById('user-password-field').hidden = wg;
      const password = user.elements.namedItem('password'); password.disabled = wg; password.required = !wg;
      document.getElementById('shared-ppp-field').hidden = wg;
      document.getElementById('shared-ppp-help').hidden = wg;
      user.elements.namedItem('shared').disabled = wg;
      document.getElementById('wireguard-user-help').hidden = !wg;
    };
    select.addEventListener('change', update); update();
    user.addEventListener('submit', async event => {
      event.preventDefault();
      const errorBox = document.getElementById('user-errors'); errorBox.hidden = true;
      const button = user.querySelector('button[type="submit"]'); button.disabled = true;
      try { const job = await send(user, user.action); location.assign('/?page=job&id=' + encodeURIComponent(job.id)); }
      catch (error) { errorBox.textContent = error.message; errorBox.hidden = false; errorBox.focus(); }
      finally { button.disabled = false; }
    });
  }
})();
