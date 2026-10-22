/* CPA WebUI storage interoperability. Official source revisions and notices:
 * docs/SOURCES.md and docs/third-party/cpa-webui-LICENSE.txt.
 * This adapter only reads the host's remembered session; it never stores keys.
 */
(() => {
  'use strict';
  const wire = Object.freeze({
    auth: 'cli-proxy-auth',
    login: 'isLoggedIn',
    encoding: 'enc::v1::',
    salt: 'cli-proxy-api-webui::secure-storage',
    storageLimit: 65536,
    keyLimit: 8192
  });
  const record = value => value !== null && typeof value === 'object' && !Array.isArray(value);

  function pluginMount(win) {
    const route = /^(.+?)?\/v(?:0|8)\/resource\/plugins\/cpa-spark\//.exec(win.location.pathname);
    return route ? route[1] || '' : null;
  }

  function decodeField(raw, win) {
    if (typeof raw !== 'string' || raw.length > wire.storageLimit) throw new Error('Invalid storage field');
    if (!raw.startsWith('enc::')) return raw;
    if (!raw.startsWith(wire.encoding)) throw new Error('Unsupported storage encoding');
    const binary = atob(raw.substring(wire.encoding.length));
    const mask = new TextEncoder().encode([wire.salt, win.location.host, win.navigator.userAgent].join('|'));
    const decoded = Uint8Array.from(binary, (character, index) => character.charCodeAt(0) ^ mask[index % mask.length]);
    return new TextDecoder('utf-8', { fatal: true }).decode(decoded);
  }

  function readField(storage, name, win, jsonOnly = false) {
    const raw = storage.getItem(name);
    if (raw === null) return null;
    const text = decodeField(raw, win);
    try { return JSON.parse(text); }
    catch (error) { if (jsonOnly) throw error; return text; }
  }

  function rememberedSettings(storage, win) {
    if (storage.getItem(wire.auth) !== null) {
      const persisted = readField(storage, wire.auth, win, true);
      if (!record(persisted)) return null;
      const settings = Object.hasOwn(persisted, 'state') ? persisted.state : persisted;
      return record(settings) && settings.rememberPassword === true ? settings : null;
    }
    // The official WebUI also restores pre-Zustand storage fields.
    return {
      apiBase: readField(storage, 'apiBase', win) || readField(storage, 'apiUrl', win),
      managementKey: readField(storage, 'managementKey', win)
    };
  }

  function belongsToPlugin(base, win, mount) {
    if (typeof base !== 'string' || !base.trim()) return false;
    const endpoint = new URL(base);
    const path = endpoint.pathname.replace(/\/v(?:0|8)\/management\/?$/, '').replace(/\/+$/, '');
    return endpoint.origin === win.location.origin && path === mount &&
      !endpoint.username && !endpoint.password && !endpoint.search && !endpoint.hash;
  }

  function savedConnection(win) {
    try {
      const mount = pluginMount(win);
      if (mount === null) return null;
      const storage = win.localStorage;
      if (storage.getItem(wire.login) !== 'true') return null;
      const settings = rememberedSettings(storage, win);
      if (!settings || !belongsToPlugin(settings.apiBase, win, mount)) return null;
      const key = settings.managementKey;
      if (typeof key !== 'string' || key.length > wire.keyLimit || /[\r\n]/.test(key)) return null;
      return key.trim() ? { managementKey: key.trim() } : null;
    } catch { return null; }
  }

  globalThis.CPAPluginSession = Object.freeze({
    savedConnection,
    resourcePrefix: win => pluginMount(win) ?? '',
    loginURL: win => `${win.location.origin}${pluginMount(win) ?? ''}/management.html#/login`
  });
})();
