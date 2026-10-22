// Read CPA WebUI's saved session; never write or duplicate its credentials.
// Wire format follows official secureStorage.ts, encryption.ts and useAuthStore.ts.
(() => {
  'use strict';
  function resourcePrefix(win) {
    const match = win.location.pathname.match(/^(.*?)\/v(?:0|8)\/resource\/plugins\/cpa-spark\//);
    return match ? match[1] : '';
  }
  function read(win, name) {
    try {
      let raw = win.localStorage.getItem(name);
      if (typeof raw !== 'string' || raw.length > 65536) return null;
      if (raw.startsWith('enc::')) {
        if (!raw.startsWith('enc::v1::')) return null;
        const bytes = Uint8Array.from(atob(raw.slice(9)), c => c.charCodeAt(0));
        const mask = new TextEncoder().encode(`cli-proxy-api-webui::secure-storage|${win.location.host}|${win.navigator.userAgent}`);
        for (let i = 0; i < bytes.length; i++) bytes[i] ^= mask[i % mask.length];
        raw = new TextDecoder().decode(bytes);
      }
      try { return JSON.parse(raw); } catch { return name === 'cli-proxy-auth' ? null : raw; }
    } catch { return null; }
  }
  function savedConnection(win) {
    try {
      if (win.localStorage.getItem('isLoggedIn') !== 'true') return null;
      const stored = read(win, 'cli-proxy-auth');
      const state = stored && typeof stored === 'object' ? stored.state || stored : null;
      if (state && state.rememberPassword !== true) return null;
      const base = state ? state.apiBase : read(win, 'apiBase') || read(win, 'apiUrl');
      const key = state ? state.managementKey : read(win, 'managementKey');
      if (typeof base !== 'string' || typeof key !== 'string' || !key.trim() || key.length > 8192 || /[\r\n]/.test(key)) return null;
      const url = new URL(base);
      const mount = url.pathname.replace(/\/v(?:0|8)\/management\/?$/, '').replace(/\/+$/, '');
      if (url.origin !== win.location.origin || url.search || url.hash || url.username || url.password || mount !== resourcePrefix(win)) return null;
      return { managementKey: key.trim() };
    } catch { return null; }
  }
  globalThis.CPAPluginSession = Object.freeze({
    savedConnection, resourcePrefix,
    loginURL: win => `${win.location.origin}${resourcePrefix(win)}/management.html#/login`
  });
})();
