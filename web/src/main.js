import './core.css';

const app = document.querySelector('#app');
const tabs = {
  dashboard: 'Dashboard',
  'balans-planning': 'Balans & Planning',
  'inkomsten-uitgaven': 'Inkomsten & Uitgaven',
  documenten: 'Documenten',
  import: 'Import',
  configuratie: 'Configuratie',
};
let activeTab = Object.hasOwn(tabs, window.location.hash.slice(1)) ? window.location.hash.slice(1) : 'balans-planning';
document.title = `${tabs[activeTab]} · HoHoHo`;
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || '';
const types = { asset: 'Bezit', debt: 'Schuld', expense: 'Last', reserve: 'Reservering' };
const objectSections = {
  asset: { title: 'Bezittingen', create: 'Nieuw bezit', id: 'balance-assets' },
  debt: { title: 'Schulden', create: 'Nieuwe schuld', id: 'balance-debts' },
  expense: { title: 'Lasten', create: 'Nieuwe last', id: 'balance-expenses' },
  reserve: { title: 'Reserveringen', create: 'Nieuwe reservering', id: 'balance-reserves' },
};
const configurationSections = {
  application: 'Applicatie Configuratie',
  database: 'Database configuratie',
  bankimport: 'Bankimport Configuratie',
  backup: 'Backup en Recovery',
};
const defaultTransactionAIModel = 'Qwen3.6-35B-A3B-Q4_K_M';
const money = new Intl.NumberFormat('nl-NL', { style: 'currency', currency: 'EUR' });
const transactionStatuses = { I: 'Geimporteerd', A: 'AI geclassificeerd', F: 'AI mislukt', U: 'Handmatig geclassificeerd' };
const documentStatuses = transactionStatuses;
let objects = [];
let values = [];
let relations = [];
let objectTransactions = [];
let summary = {};
let selectedID = null;
let objectTypeFocus = 'all';
let objectEditorOpen = false;
let objectDialogMode = '';
let message = '';
let requestID = 0;
let transactions = [];
let transactionTotals = { total: 0, incomeCents: 0, expenseCents: 0, balanceCents: 0 };
let transactionViewFilter = '';
let transactionFromFilter = '';
let transactionToFilter = '';
let transactionSearchFilter = '';
let transactionSort = 'date';
let transactionOrder = 'desc';
let transactionOffset = 0;
let transactionDraft = null;
let transactionEditRequestID = 0;
const transactionPageSize = 25;
const dashboardMonths = ['Jan', 'Feb', 'Mrt', 'Apr', 'Mei', 'Jun', 'Jul', 'Aug', 'Sep', 'Okt', 'Nov', 'Dec'];
let dashboard = null;
let dashboardYear = new Date().getFullYear();
let dashboardMonth = 0;
let dashboardLoading = true;
let dashboardError = '';
let dashboardRequestID = 0;
let configuration = null;
let configurationLoading = true;
let configurationError = '';
let configurationRequestID = 0;
let configurationFocus = 'application';
let transactionAIModels = [];
let transactionAIModelsLoading = false;
let transactionAIModelsError = '';
let backupStatus = null;
let backupSnapshots = [];
let backupMessage = '';
let bankConfigs = [];
let selectedBankConfig = null;
let bankConfigMessage = '';
let bankImportPreview = null;
let bankImportMessage = '';
let bankImportFile = null;
let bankImportSelectedConfig = '';
let documentMessage = '';
let documents = [];
let documentsLoading = true;
let documentsError = '';
let documentsRequestID = 0;
let documentUploadOpen = false;
let documentHelpOpen = false;
let selectedDocumentID = null;

function escapeHtml(value) {
  return String(value ?? '').replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]);
}

async function api(path, options = {}) {
  const response = await fetch(`${apiBaseUrl}/api/v1${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options.headers },
  });
  if (!response.ok) {
    const result = await response.json().catch(() => ({}));
    throw new Error(result.error || `Verzoek mislukt (${response.status})`);
  }
  return response.status === 204 ? null : response.json();
}

async function refresh() {
  const currentRequest = ++requestID;
  ++dashboardRequestID;
  const selection = selectedID;
  const transactionQuery = new URLSearchParams({ limit: transactionPageSize, offset: transactionOffset });
  if (transactionViewFilter === 'income' || transactionViewFilter === 'expense') transactionQuery.set('direction', transactionViewFilter);
  if (transactionViewFilter === 'hidden') transactionQuery.set('visibility', 'hidden');
  if (transactionFromFilter) transactionQuery.set('from', transactionFromFilter);
  if (transactionToFilter) transactionQuery.set('to', transactionToFilter);
  if (transactionSearchFilter.trim()) transactionQuery.set('search', transactionSearchFilter.trim());
  transactionQuery.set('sort', transactionSort);
  transactionQuery.set('order', transactionOrder);
  const [nextObjects, nextSummary, nextTransactions, nextDocuments] = await Promise.all([
    api('/financial-objects'), api('/summary'), api(`/transactions?${transactionQuery}`), loadDocumentsData(),
  ]);
  let nextValues = [];
  let nextRelations = [];
  let nextObjectTransactions = [];
  if (selection != null && nextObjects.some((object) => object.id === selection)) {
    [nextValues, nextRelations, nextObjectTransactions] = await Promise.all([
      api(`/financial-objects/${selection}/values`),
      api(`/financial-objects/${selection}/relations`),
      api(`/transactions?financialObjectId=${selection}&limit=10&sort=date&order=desc`),
    ]);
  }
  if (currentRequest !== requestID) return;
  objects = nextObjects;
  summary = nextSummary;
  selectedID = nextObjects.some((object) => object.id === selection) ? selection : null;
  values = nextValues;
  relations = nextRelations;
  objectTransactions = nextObjectTransactions.items || [];
  transactions = nextTransactions.items;
  transactionTotals = nextTransactions;
  documents = nextDocuments;
  render();
  await Promise.all([loadDashboard(), loadConfiguration()]);
}

function render() {
  const selected = objects.find((object) => object.id === selectedID);
  app.innerHTML = `<div class="core-shell">
    <nav class="core-navigation" aria-label="Hoofdnavigatie">${Object.entries(tabs).map(([id, label]) => `<a href="#${id}" data-tab="${id}" ${activeTab === id ? 'aria-current="page"' : ''}>${escapeHtml(label)}</a>`).join('')}</nav>
    <p id="message" class="error" role="alert">${escapeHtml(message)}</p>
    <section id="dashboard" class="core-tab-section" aria-labelledby="dashboard-heading" ${activeTab === 'dashboard' ? '' : 'hidden'}>
      <header><p class="eyebrow">House Hold Hosting · v3</p><h1 id="dashboard-heading">Dashboard</h1>
        <p>Je balans, planning en geldstromen voor de gekozen periode.</p></header>
      <div id="dashboard-content">${renderDashboard()}</div>
    </section>
    <section id="balans-planning" class="core-tab-section" aria-labelledby="balans-heading" ${activeTab === 'balans-planning' ? '' : 'hidden'}>
    <header><p class="eyebrow">House Hold Hosting · v3</p><h1 id="balans-heading">Balans &amp; Planning</h1>
      <p>Beheer je financiële objecten: bezittingen, schulden, lasten en reserveringen.</p></header>
    <p class="balance-overview">${summary.financialObjects ?? 0} financiële objecten · ${summary.objectValues ?? 0} waarderingen · ${summary.objectRelations ?? 0} relaties</p>
    <div class="balance-toolbar">
      <div class="balance-subnavigation" role="group" aria-label="Objectsoorten"><button type="button" class="period-button ${objectTypeFocus === 'all' ? 'active' : ''}" data-action="object-type" data-type="all" aria-pressed="${objectTypeFocus === 'all'}">Alle objecten</button>${Object.entries(types).map(([type, label]) => `<button type="button" class="period-button ${objectTypeFocus === type ? 'active' : ''}" data-action="object-type" data-type="${type}" aria-pressed="${objectTypeFocus === type}">${label}</button>`).join('')}</div>
      <div class="balance-create-actions" role="group" aria-label="Nieuw financieel object">${Object.entries(objectSections).map(([type, section]) => `<button type="button" class="balance-create-button ${type}" data-action="new" data-type="${type}">${section.create}</button>`).join('')}</div>
    </div>
    <div class="balance-object-grid ${objectTypeFocus === 'all' ? '' : 'single'}">${Object.entries(objectSections).filter(([type]) => objectTypeFocus === 'all' || objectTypeFocus === type).map(([type, section]) => renderObjectFrame(type, section)).join('')}</div>
    ${objectEditorOpen ? renderObjectEditor(selected) : ''}
    </section>
    <section id="inkomsten-uitgaven" class="core-tab-section" aria-labelledby="inkomsten-heading" ${activeTab === 'inkomsten-uitgaven' ? '' : 'hidden'}>
      <header><p class="eyebrow">House Hold Hosting · v3</p><h1 id="inkomsten-heading">Inkomsten &amp; Uitgaven</h1>
        <p>Beheer je ontvangsten en betalingen en verbind ze met financiële objecten.</p></header>
      ${renderTransactions()}
    </section>
    <section id="documenten" class="core-tab-section" aria-labelledby="documenten-heading" ${activeTab === 'documenten' ? '' : 'hidden'}>
      <header><p class="eyebrow">House Hold Hosting · v3</p><h1 id="documenten-heading">Documenten</h1>
        <p>Documenten en hun koppelingen aan financiële objecten.</p></header>
      <div id="document-content">${renderDocuments()}</div>
    </section>
    <section id="import" class="core-tab-section" aria-labelledby="import-heading" ${activeTab === 'import' ? '' : 'hidden'}>
      <header><p class="eyebrow">House Hold Hosting · v3</p><h1 id="import-heading">Import</h1>
        <p>Bankgegevens omzetten naar genormaliseerde transacties.</p></header>
      <div id="bank-import-content">${renderBankImport()}</div>
    </section>
    <section id="configuratie" class="core-tab-section" aria-labelledby="configuratie-heading" ${activeTab === 'configuratie' ? '' : 'hidden'}>
      <header><p class="eyebrow">House Hold Hosting · v3</p><h1 id="configuratie-heading">Configuratie</h1>
        <p>Runtime-instellingen, opslagstatus en voorbereiding voor backup en herstel.</p></header>
      <div id="configuration-content">${renderConfiguration()}</div>
    </section>
    <footer>SQLite · één huishouden. Inkomsten en uitgaven worden handmatig vastgelegd; bankimport en documenten volgen later.</footer>
  </div>`;
}

function renderBankImport() {
  const validConfigs = bankConfigs.filter((item) => item.status?.valid);
  const preview = bankImportPreview;
  const selectedConfig = bankImportSelectedConfig || preview?.bankConfig || validConfigs[0]?.slug || '';
  const selectedFile = bankImportFile ? `<p class="muted">Geselecteerd bestand: <strong>${escapeHtml(bankImportFile.name)}</strong>. Je hoeft dit na preview niet opnieuw te kiezen.</p>` : '';
  return `<section class="core-panel core-prepared" aria-labelledby="bank-import-heading"><h2 id="bank-import-heading">Banktransacties importeren</h2>
    <p class="muted">Kies een bankconfiguratie en CSV-export. Preview is optioneel; definitief importeren kan direct en gebruikt daarna de Transactie-AI voor objecttoewijzing.</p>
    ${bankImportMessage ? `<p class="config-notice">${escapeHtml(bankImportMessage)}</p>` : ''}
    <form id="bank-import-form" class="configuration-form">
      <label>Bankconfiguratie <select name="bankConfig" required>${validConfigs.map((item) => `<option value="${escapeHtml(item.slug)}" ${selectedConfig === item.slug ? 'selected' : ''}>${escapeHtml(item.bank.name)} (${escapeHtml(item.slug)}.yaml)</option>`).join('')}</select></label>
      <label>Bankbestand <input name="file" type="file" accept=".csv,text/csv" /></label>
      ${selectedFile}
      <div class="core-actions"><button type="submit" ${validConfigs.length && bankImportFile ? '' : 'disabled'}>Preview maken</button><button type="button" data-action="import-bank-transactions" ${validConfigs.length && bankImportFile ? '' : 'disabled'}>Transacties importeren</button></div>
    </form>
    ${validConfigs.length ? '' : '<p class="muted">Maak eerst een geldige bankconfiguratie aan onder Configuratie → Bankimport Configuratie.</p>'}
    ${preview ? `<section class="core-detail"><h3>Preview ${escapeHtml(preview.bankName)}</h3><p class="muted">${preview.total} transactieregels gelezen. Controleer datum, bedrag, omschrijving en tegenpartij voordat je importeert.</p>
      <div class="core-table-scroll"><table class="core-transaction-table"><thead><tr><th>Rij</th><th>Datum</th><th>Bedrag</th><th>Omschrijving</th><th>Tegenpartij</th></tr></thead><tbody>${preview.rows.map((row) => `<tr><td>${row.row}</td><td>${escapeHtml(row.transaction.transaction_date)}</td><td class="${row.transaction.amount_cents > 0 ? 'income' : 'expense'}">${money.format(row.transaction.amount_cents / 100)}</td><td>${escapeHtml(row.transaction.description)}</td><td>${escapeHtml(row.transaction.counterparty)}</td></tr>`).join('')}</tbody></table></div>
    </section>` : ''}
  </section>`;
}

function renderConfiguration() {
  if (configurationLoading) return '<section class="core-panel core-prepared"><h2>Configuratie laden</h2><p class="muted">Opslagstatus en runtime-instellingen worden opgehaald.</p></section>';
  if (configurationError) return `<section class="core-panel core-prepared"><h2>Configuratie niet beschikbaar</h2><p class="error">${escapeHtml(configurationError)}</p></section>`;
  const cfg = configuration;
  const databaseSize = cfg.database.sizeBytes ? `${(cfg.database.sizeBytes / 1024 / 1024).toFixed(2).replace('.', ',')} MB` : 'Nog niet bekend';
  const statusBadge = (ok, label) => `<span class="config-status ${ok ? 'ok' : 'warning'}">${escapeHtml(label)}</span>`;
  return `<div class="configuration-toolbar" role="group" aria-label="Configuratieonderdelen">${Object.entries(configurationSections).map(([id, label]) => `<button type="button" class="period-button ${configurationFocus === id ? 'active' : ''}" data-action="configuration-section" data-section="${id}" aria-pressed="${configurationFocus === id}">${escapeHtml(label)}</button>`).join('')}</div>
  <div class="configuration-grid single">
    ${configurationFocus === 'database' ? renderDatabaseConfiguration(cfg, databaseSize, statusBadge) : ''}
    ${configurationFocus === 'backup' ? renderBackupConfiguration(cfg, statusBadge) : ''}
    ${configurationFocus === 'application' ? renderApplicationConfiguration(cfg) : ''}
    ${configurationFocus === 'bankimport' ? renderBankimportConfiguration() : ''}
  </div>`;
}

function renderDatabaseConfiguration(cfg, databaseSize, statusBadge) {
  return `<section class="core-panel configuration-panel" aria-labelledby="database-config-heading">
      <div class="core-heading"><h2 id="database-config-heading">Databases en opslag</h2>${statusBadge(cfg.database.status === 'ok', cfg.database.status === 'ok' ? 'SQLite bereikbaar' : 'SQLite fout')}</div>
      <p class="muted">V3 gebruikt één vaste opslagcombinatie: SQLite voor gestructureerde data en Garage/S3 voor documentbytes. S3-wijzigingen worden opgeslagen en gelden na een herstart.</p>
      ${cfg.restartRequired ? '<p class="config-notice">Configuratie opgeslagen. Herstart de applicatie om de gewijzigde S3-instellingen actief te maken.</p>' : ''}
      <dl class="configuration-list">
        <div><dt>Database backend</dt><dd>SQLite</dd></div>
        <div><dt>Databasepad</dt><dd><code>${escapeHtml(cfg.database.path)}</code></dd></div>
        <div><dt>Schemaversie</dt><dd>${cfg.database.schemaVersion}</dd></div>
        <div><dt>Databasegrootte</dt><dd>${databaseSize}</dd></div>
        <div><dt>S3 toegang</dt><dd>${cfg.objectStorage.accessKeySet ? 'Access key aanwezig' : 'Access key ontbreekt'} · ${cfg.objectStorage.secretKeySet ? 'Secret key aanwezig' : 'Secret key ontbreekt'}</dd></div>
        <div><dt>Objectopslagstatus</dt><dd>${statusBadge(cfg.objectStorage.ready, cfg.objectStorage.statusMessage || (cfg.objectStorage.ready ? 'Bereikbaar' : 'Niet bereikbaar'))}</dd></div>
      </dl>
      <form id="configuration-form" class="configuration-form">
        <h3>Garage/S3 configureren</h3>
        <label>Endpoint <input name="s3Endpoint" value="${escapeHtml(cfg.objectStorage.endpoint)}" placeholder="garage:3900" /></label>
        <label>Bucket <input name="s3Bucket" value="${escapeHtml(cfg.objectStorage.bucket)}" required /></label>
        <label>Regio <input name="s3Region" value="${escapeHtml(cfg.objectStorage.region || 'garage')}" required /></label>
        <div class="transaction-fields">
          <label>Access key <input name="s3AccessKey" type="password" autocomplete="new-password" placeholder="${cfg.objectStorage.accessKeySet ? 'Ongewijzigd laten' : 'Invullen'}" /></label>
          <label>Secret key <input name="s3SecretKey" type="password" autocomplete="new-password" placeholder="${cfg.objectStorage.secretKeySet ? 'Ongewijzigd laten' : 'Invullen'}" /></label>
        </div>
        <div class="configuration-checks">
          <label><input name="s3UseTLS" type="checkbox" ${cfg.objectStorage.tls ? 'checked' : ''} /> TLS gebruiken</label>
          <label><input name="s3PathStyle" type="checkbox" ${cfg.objectStorage.pathStyle ? 'checked' : ''} /> Path-style requests</label>
          <label><input name="s3CreateBucket" type="checkbox" ${cfg.objectStorage.createBucket ? 'checked' : ''} /> Bucket aanmaken indien nodig</label>
        </div>
        <div class="core-actions"><button type="submit">S3-configuratie opslaan</button></div>
      </form>
    </section>`;
}

function renderApplicationConfiguration(cfg) {
  const transactionAIURL = cfg.runtime.transactionAiUrl || '';
  const transactionAIModel = cfg.runtime.transactionAiModel || transactionAIModels[0] || defaultTransactionAIModel;
  const availableModels = transactionAIModels.length ? transactionAIModels : [transactionAIModel];
  const modelOptions = [...new Set([transactionAIModel, ...availableModels])].map((model) => `<option value="${escapeHtml(model)}" ${model === transactionAIModel ? 'selected' : ''}>${escapeHtml(model)}</option>`).join('');
  const modelStatus = transactionAIModelsLoading ? 'Modellen ophalen...' : transactionAIModelsError || (transactionAIModels.length ? `${transactionAIModels.length} modellen gevonden.` : 'Vul de URL in en haal de beschikbare modellen op.');
  return `<section class="core-panel configuration-panel" aria-labelledby="runtime-config-heading">
      <h2 id="runtime-config-heading">Applicatie</h2>
      <dl class="configuration-list">
        <div><dt>Omgeving</dt><dd>${escapeHtml(cfg.runtime.environment)}</dd></div>
        <div><dt>HTTP-adres</dt><dd><code>${escapeHtml(cfg.runtime.httpAddress)}</code></dd></div>
        <div><dt>Frontendmap</dt><dd><code>${escapeHtml(cfg.runtime.webDirectory)}</code></dd></div>
        <div><dt>Authenticatie</dt><dd>${cfg.runtime.authenticationEnabled ? 'Ingeschakeld' : 'Uitgeschakeld'}</dd></div>
        <div><dt>CORS</dt><dd>${cfg.runtime.corsConfigured ? 'Geconfigureerd' : 'Niet ingesteld'}</dd></div>
        <div><dt>PDF/OCR-AI</dt><dd>${cfg.runtime.pdfAiConfigured ? 'Geconfigureerd' : 'Niet ingesteld'}</dd></div>
        <div><dt>Transactie-AI</dt><dd>${cfg.runtime.transactionAiConfigured ? 'Geconfigureerd' : 'Niet ingesteld'}</dd></div>
      </dl>
      ${cfg.restartRequired ? '<p class="config-notice">Configuratie opgeslagen. Herstart de applicatie om gewijzigde serverkoppelingen actief te maken.</p>' : ''}
      <form id="application-config-form" class="configuration-form">
        <h3>AI-koppelingen</h3>
        <label>PDF/OCR-AI URL <input name="pdfAiUrl" value="${escapeHtml(cfg.runtime.pdfAiUrl || '')}" placeholder="http://pdf-ai:8090" /></label>
        <label>Transactieclassificatie-AI URL <input name="transactionAiUrl" value="${escapeHtml(transactionAIURL)}" placeholder="http://192.168.178.215:8080/v1/chat/completions" /></label>
        <div id="transaction-ai-model-row" class="configuration-inline" ${transactionAIURL ? '' : 'hidden'}>
          <label>Transactie-AI model <select name="transactionAiModel">${modelOptions}</select></label>
          <button type="button" data-action="load-transaction-ai-models">Modellen ophalen</button>
        </div>
        <p id="transaction-ai-model-status" class="muted" ${transactionAIURL ? '' : 'hidden'}>${escapeHtml(modelStatus)}</p>
        <p class="muted">De PDF/OCR-AI wordt gebruikt voor documentverwerking. De transactie-AI is bedoeld voor automatische classificatie van banktransacties.</p>
        <div class="core-actions"><button type="submit">Applicatieconfiguratie opslaan</button></div>
      </form>
    </section>`;
}

function renderBankimportConfiguration() {
  const selected = selectedBankConfig || bankConfigs[0] || null;
  return `<section class="core-panel configuration-panel" aria-labelledby="bankimport-config-heading">
    <div class="core-heading"><h2 id="bankimport-config-heading">Bankimport Configuratie</h2><button type="button" data-action="new-bank-config">Nieuwe bankconfig</button></div>
    <p class="muted">Per bank staat één YAML-bestand in <code>config/banks/</code>. De mapping vertaalt bronvelden naar het interne transactiemodel: <code>transaction_date</code>, <code>amount_cents</code>, <code>description</code> en <code>counterparty</code>.</p>
    ${bankConfigMessage ? `<p class="config-notice">${escapeHtml(bankConfigMessage)}</p>` : ''}
    <div class="bank-config-layout">
      <aside class="bank-config-list" aria-label="Bankconfiguraties">
        ${bankConfigs.length ? bankConfigs.map((item) => `<button type="button" class="object-button ${selected?.slug === item.slug ? 'selected' : ''}" data-action="select-bank-config" data-slug="${escapeHtml(item.slug)}"><span><strong>${escapeHtml(item.bank?.name || item.slug)}</strong><small>${escapeHtml(item.slug)}.yaml · ${item.status?.valid ? 'geldig' : 'ongeldig'}</small></span></button>`).join('') : '<p class="muted">Nog geen bankconfiguraties.</p>'}
      </aside>
      <form id="bank-config-form" class="configuration-form">
        <label>Bestandsnaam <input name="slug" value="${escapeHtml(selected?.slug || '')}" placeholder="sparda" pattern="[a-z0-9][a-z0-9_-]*" required /></label>
        <label>YAML-configuratie <textarea name="rawYaml" rows="18" spellcheck="false" required>${escapeHtml(selected?.rawYaml || '')}</textarea></label>
        ${selected?.status?.errors?.length ? `<p class="error">${selected.status.errors.map(escapeHtml).join('\n')}</p>` : ''}
        <div class="core-actions"><button type="submit">Bankconfig opslaan</button>${selected?.slug ? `<button type="button" class="secondary" data-action="delete-bank-config" data-slug="${escapeHtml(selected.slug)}">Verwijderen</button>` : ''}</div>
      </form>
    </div>
  </section>`;
}

function renderBackupConfiguration(cfg, statusBadge) {
  const status = backupStatus || {};
  const policy = status.policy || { schedule: '02:30', keepDaily: 14, keepWeekly: 8, keepMonthly: 24 };
  const ready = cfg.backupRecovery.available;
  return `<section class="core-panel configuration-panel" aria-labelledby="backup-config-heading">
    <div class="core-heading"><h2 id="backup-config-heading">Backup en recovery</h2>${statusBadge(ready && status.configured, ready ? (status.configured ? 'Geconfigureerd' : 'Beschikbaar') : 'Niet beschikbaar')}</div>
    <p class="muted">${escapeHtml(cfg.backupRecovery.message)}</p>
    ${backupMessage ? `<p class="config-notice">${escapeHtml(backupMessage)}</p>` : ''}
    <form id="backup-form" class="configuration-form">
      <label>Restic repository <input name="repository" value="${escapeHtml(status.repository)}" placeholder="s3:garage:hohoho-backups of rclone:remote:path" ${ready ? '' : 'disabled'} required /></label>
      <label>Rclone config <textarea name="rcloneConfig" rows="7" placeholder="[garage]\ntype = s3\nprovider = Minio\n..." ${ready ? '' : 'disabled'}></textarea><small>Laat leeg om bestaande rclone config te behouden.</small></label>
       <label>Restic wachtwoord <input name="resticPassword" type="password" autocomplete="new-password" placeholder="${status.resticPasswordSet ? 'Ongewijzigd laten' : 'Invullen'}" ${ready ? '' : 'disabled'} /></label>
       <label>Dagelijks tijdstip <input name="schedule" type="time" value="${escapeHtml(policy.schedule)}" ${ready ? '' : 'disabled'} required /></label>
       <label>Dagelijkse snapshots <input name="keepDaily" type="number" min="1" max="365" value="${escapeHtml(policy.keepDaily)}" ${ready ? '' : 'disabled'} required /></label>
       <label>Wekelijkse snapshots <input name="keepWeekly" type="number" min="1" max="520" value="${escapeHtml(policy.keepWeekly)}" ${ready ? '' : 'disabled'} required /></label>
       <label>Maandelijkse snapshots <input name="keepMonthly" type="number" min="1" max="120" value="${escapeHtml(policy.keepMonthly)}" ${ready ? '' : 'disabled'} required /></label>
      <div class="core-actions"><button type="submit" ${ready ? '' : 'disabled'}>Opslaan en controleren</button><button type="button" class="secondary" data-action="backup-validate" ${ready && status.configured ? '' : 'disabled'}>Opnieuw controleren</button><button type="button" data-action="backup-run" ${ready && status.validated && !status.running ? '' : 'disabled'}>Backup starten</button></div>
    </form>
    <dl class="configuration-list">
      <div><dt>Status</dt><dd>${status.running ? 'Backup draait' : (status.validated ? 'Configuratie gevalideerd' : 'Nog niet gevalideerd')}</dd></div>
       <div><dt>Laatste snapshot</dt><dd>${escapeHtml(status.lastSnapshotId || 'Geen')}</dd></div>
       <div><dt>Laatste controle</dt><dd>${escapeHtml(status.lastValidatedAt || 'Geen')}</dd></div>
       <div><dt>Laatste restore-proef</dt><dd>${escapeHtml(status.lastDrillAt || 'Geen')}</dd></div>
      <div><dt>Laatste fout</dt><dd>${escapeHtml(status.lastError || status.recoveryError || 'Geen')}</dd></div>
      <div><dt>Recovery</dt><dd>${escapeHtml(status.recoveryState || 'Geen herstel gepland')}</dd></div>
    </dl>
    <section class="core-detail"><div class="core-heading"><h3>Snapshots</h3><button type="button" class="secondary" data-action="backup-snapshots" ${ready && status.configured ? '' : 'disabled'}>Snapshots verversen</button></div>
      ${backupSnapshots.length ? `<ul class="core-detail-list">${backupSnapshots.map((snapshot) => `<li><span><strong>${escapeHtml(snapshot.shortId)}</strong> · ${escapeHtml(snapshot.time)}</span><button type="button" class="secondary" data-action="select-recovery-snapshot" data-id="${escapeHtml(snapshot.id)}">Kies voor herstel</button></li>`).join('')}</ul>` : '<p class="muted">Nog geen snapshots geladen.</p>'}
      <form id="recovery-form" class="configuration-form">
        <label>Snapshot ID <input name="snapshotId" placeholder="Snapshot ID" ${ready ? '' : 'disabled'} required /></label>
        <label>Bevestiging <input name="confirmation" placeholder="Typ HERSTEL" ${ready ? '' : 'disabled'} required /></label>
        <button type="submit" class="secondary" ${ready ? '' : 'disabled'}>Recovery plannen</button>
      </form>
    </section>
  </section>`;
}

function updateConfiguration() {
  const root = document.querySelector('#configuration-content');
  if (root) root.innerHTML = renderConfiguration();
}

function updateBankImport() {
  const root = document.querySelector('#bank-import-content');
  if (root) root.innerHTML = renderBankImport();
}

function renderDocuments() {
  const status = configuration?.objectStorage;
  return `<section class="core-panel" aria-labelledby="document-actions-heading"><div class="core-heading"><h2 id="document-actions-heading">Documenten</h2><div class="core-actions compact"><button type="button" class="help-button" data-action="document-help" aria-label="Uitleg over documenten">?</button><button type="button" class="secondary small-action" data-action="new-document" ${status && !status.ready ? 'disabled' : ''}>Nieuw document</button></div></div>
    ${status && !status.ready ? `<p class="config-notice">${escapeHtml(status.statusMessage || 'S3/Garage is niet bereikbaar; uploaden is uitgeschakeld.')}</p>` : ''}
    ${documentMessage ? `<p class="config-notice">${escapeHtml(documentMessage)}</p>` : ''}
  </section>
  <section class="core-panel core-prepared" aria-labelledby="document-list-heading"><div class="core-heading"><h2 id="document-list-heading">Bestaande documenten</h2><button type="button" class="secondary" data-action="refresh-documents">Verversen</button></div>
    ${renderDocumentList()}
  </section>
  ${renderDocumentViewer()}
  ${documentUploadOpen ? renderDocumentUploadDialog(status) : ''}
  ${documentHelpOpen ? renderDocumentHelpDialog() : ''}`;
}

function renderDocumentUploadDialog(status) {
  return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="document-upload-heading"><div class="core-heading"><h2 id="document-upload-heading">Nieuw document</h2><button type="button" class="secondary" data-action="close-document-upload">Sluiten</button></div>
    <form id="document-upload-form" class="configuration-form">
      <label>Bestand <input name="file" type="file" required /></label>
      <div class="core-actions"><button type="submit" ${status && !status.ready ? 'disabled' : ''}>Uploaden</button></div>
    </form>
  </section></div>`;
}

function renderDocumentHelpDialog() {
  return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="document-help-heading"><div class="core-heading"><h2 id="document-help-heading">Documenten</h2><button type="button" class="secondary" data-action="close-document-help">Sluiten</button></div>
    <p>Upload slaat alleen het bestand op in Garage/S3 en metadata in SQLite.</p>
    <p>Classificatie start je daarna met <strong>AI</strong> of <strong>Handmatig classificeren</strong>.</p>
  </section></div>`;
}

function renderDocumentList() {
  if (documentsLoading) return '<p class="muted">Documenten laden...</p>';
  if (documentsError) return `<p class="error">${escapeHtml(documentsError)}</p>`;
  if (!documents.length) return '<p class="muted">Nog geen documenten opgeslagen.</p>';
  return `<ul class="document-list">${documents.map((item) => {
    return `<li><div><strong>${escapeHtml(item.filename)}</strong><small>Toegevoegd: ${escapeHtml(item.createdAt || '')}</small></div><span class="config-status ${item.status === 'F' ? 'warning' : 'ok'}">${escapeHtml(documentStatuses[item.status] || item.status || 'Onbekend')}</span><div class="document-actions"><button type="button" class="secondary" data-action="open-document" data-id="${item.id}">Openen</button><button type="button" class="secondary" data-action="analyze-document" data-id="${item.id}">AI</button><button type="button" class="secondary" data-action="manual-document" data-id="${item.id}">Handmatig classificeren</button><button type="button" data-action="delete-document" data-id="${item.id}">Verwijderen</button></div></li>`;
  }).join('')}</ul>`;
}

function renderDocumentViewer() {
  if (selectedDocumentID == null) return '';
  const item = documents.find((document) => document.id === selectedDocumentID);
  if (!item) return '';
  const analysis = item.analysis || null;
  const summary = analysisSummary(analysis);
  return `<section class="core-panel core-prepared document-viewer" aria-labelledby="document-viewer-heading">
    <div class="core-heading"><div><h2 id="document-viewer-heading">${escapeHtml(item.filename)}</h2><span class="config-status ${item.status === 'F' ? 'warning' : 'ok'}">${escapeHtml(documentStatuses[item.status] || item.status || 'Onbekend')}</span></div><button type="button" class="secondary" data-action="close-document-viewer">Sluiten</button></div>
    <div class="document-insights">
      <section><h3>Vertaling</h3><p>${escapeHtml(item.translatedText || 'Geen vertaling beschikbaar.')}</p></section>
      <section><h3>Analyse</h3><p>${escapeHtml(summary || 'Geen analyse beschikbaar.')}</p></section>
      <section><h3>Gegevens</h3>${analysis ? `<pre>${escapeHtml(JSON.stringify(analysis, null, 2))}</pre>` : '<p>Geen gegevens beschikbaar.</p>'}</section>
    </div>
    <iframe class="document-frame" title="${escapeHtml(item.filename)}" src="${apiBaseUrl}/api/v1/documents/${item.id}/content"></iframe>
  </section>`;
}

function analysisSummary(analysis) {
  if (!analysis || typeof analysis !== 'object') return '';
  for (const key of ['summary', 'samenvatting', 'description', 'omschrijving']) {
    if (typeof analysis[key] === 'string' && analysis[key].trim()) return analysis[key].trim();
  }
  return '';
}

async function loadDocumentsData() {
  const currentRequest = ++documentsRequestID;
  documentsLoading = true;
  documentsError = '';
  try {
    const result = await api('/documents');
    if (currentRequest === documentsRequestID) documentsLoading = false;
    return result;
  } catch (error) {
    if (currentRequest === documentsRequestID) {
      documentsLoading = false;
      documentsError = error.message;
    }
    return documents;
  }
}

async function reloadDocuments() {
  documents = await loadDocumentsData();
  if (selectedDocumentID != null && !documents.some((item) => item.id === selectedDocumentID)) selectedDocumentID = null;
  updateDocuments();
}

async function loadSelectedObjectDetails(id) {
  [values, relations, objectTransactions] = [[], [], []];
  render();
  try {
    const [nextValues, nextRelations, nextObjectTransactions] = await Promise.all([
      api(`/financial-objects/${id}/values`),
      api(`/financial-objects/${id}/relations`),
      api(`/transactions?financialObjectId=${id}&limit=10&sort=date&order=desc`),
    ]);
    if (selectedID !== id) return;
    values = nextValues;
    relations = nextRelations;
    objectTransactions = nextObjectTransactions.items || [];
    render();
  } catch (error) {
    if (selectedID === id) showError(error);
  }
}

function formatBytes(value) {
  const bytes = Number(value || 0);
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1).replace('.', ',')} KiB`;
  return `${(bytes / 1024 / 1024).toFixed(1).replace('.', ',')} MiB`;
}

function updateDocuments() {
  const root = document.querySelector('#document-content');
  if (root) root.innerHTML = renderDocuments();
}

async function loadConfiguration() {
  const currentRequest = ++configurationRequestID;
  configurationLoading = true;
  configurationError = '';
  updateConfiguration();
  try {
    const result = await api('/system/configuration');
    const [backup, configs] = await Promise.all([api('/system/backup').catch(() => null), api('/bank-configs').catch(() => [])]);
    if (currentRequest !== configurationRequestID) return;
    configuration = result;
    backupStatus = backup;
    bankConfigs = configs;
    if (selectedBankConfig) selectedBankConfig = bankConfigs.find((item) => item.slug === selectedBankConfig.slug) || selectedBankConfig;
    configurationLoading = false;
    updateConfiguration();
    updateBankImport();
  } catch (error) {
    if (currentRequest !== configurationRequestID) return;
    configurationLoading = false;
    configurationError = error.message;
    updateConfiguration();
  }
}

async function loadTransactionAIModels(url) {
  const target = (url || configuration?.runtime?.transactionAiUrl || '').trim();
  if (!target) return;
  transactionAIModelsLoading = true;
  transactionAIModelsError = '';
  updateTransactionAIModelControls();
  try {
    const result = await api(`/system/transaction-ai-models?url=${encodeURIComponent(target)}`);
    transactionAIModels = result.models || [];
    transactionAIModelsError = transactionAIModels.length ? '' : 'Geen modellen gevonden op deze AI-server.';
  } catch (error) {
    transactionAIModels = [];
    transactionAIModelsError = error.message;
  } finally {
    transactionAIModelsLoading = false;
    updateTransactionAIModelControls();
  }
}

function updateTransactionAIModelControls() {
  const form = document.querySelector('#application-config-form');
  const row = document.querySelector('#transaction-ai-model-row');
  const status = document.querySelector('#transaction-ai-model-status');
  if (!form || !row || !status) return;
  const url = form.elements.transactionAiUrl.value.trim();
  row.hidden = !url;
  status.hidden = !url;
  if (!url) return;
  const select = form.elements.transactionAiModel;
  const current = select.value || configuration?.runtime?.transactionAiModel || transactionAIModels[0] || defaultTransactionAIModel;
  if (transactionAIModels.length) {
    const models = [...new Set([current, ...transactionAIModels])];
    select.innerHTML = models.map((model) => `<option value="${escapeHtml(model)}" ${model === current ? 'selected' : ''}>${escapeHtml(model)}</option>`).join('');
  }
  status.textContent = transactionAIModelsLoading ? 'Modellen ophalen...' : transactionAIModelsError || (transactionAIModels.length ? `${transactionAIModels.length} modellen gevonden.` : 'Vul de URL in en haal de beschikbare modellen op.');
}

function renderObjectFrame(type, section) {
  const items = objects.filter((object) => object.objectType === type);
  return `<section id="${section.id}" class="core-panel object-frame ${objectTypeFocus === type ? 'focused' : ''}" data-object-type="${type}" aria-labelledby="${section.id}-heading">
    <div class="core-heading"><h2 id="${section.id}-heading">${section.title}</h2><span class="object-count" aria-label="${items.length} objecten">${items.length}</span></div>
    ${items.length ? `<ul class="core-object-list">${items.map((object) => `<li>
      <button type="button" class="object-button ${object.id === selectedID ? 'selected' : ''}" data-action="select" data-id="${object.id}">
        <span><strong>${escapeHtml(object.name)}</strong>${object.description ? `<small>${escapeHtml(object.description)}</small>` : ''}</span>
        <span class="object-list-value"><strong>${object.currentValue ? money.format(object.currentValue.valueCents / 100) : 'Geen waarde'}</strong>${object.currentValue ? `<small>${escapeHtml(object.currentValue.valueDate)}</small>` : ''}</span>
      </button></li>`).join('')}</ul>` : `<p class="muted">Nog geen ${section.title.toLowerCase()}.</p>`}
  </section>`;
}

function renderObjectEditor(selected) {
  const type = selected?.objectType || (objectTypeFocus === 'all' ? 'asset' : objectTypeFocus);
  if (!selected) return renderObjectFormDialog(null, type);
  return `<section id="object-editor" class="core-panel balance-object-editor compact-detail" aria-labelledby="object-editor-heading">
    <div class="core-heading"><div><h2 id="object-editor-heading">${escapeHtml(selected.name)}</h2><span class="config-status ok">${escapeHtml(types[type])}</span></div><button type="button" class="secondary small-action" data-action="close-object-editor">Sluiten</button></div>
    <dl class="compact-detail-grid">
      <div><dt>Actuele waarde</dt><dd>${selected.currentValue ? money.format(selected.currentValue.valueCents / 100) : 'Geen waarde'}</dd></div>
      <div><dt>Waardedatum</dt><dd>${escapeHtml(selected.currentValue?.valueDate || 'Geen')}</dd></div>
      <div><dt>Omschrijving</dt><dd>${escapeHtml(selected.description || 'Geen')}</dd></div>
    </dl>
    <div class="core-actions compact"><button type="button" data-action="open-object-form">Wijzigen</button><button type="button" class="secondary" data-action="open-value-form">Waarde toevoegen</button><button type="button" class="secondary" data-action="open-relation-form">Relatie toevoegen</button><button type="button" class="secondary" data-action="delete-object" data-id="${selected.id}">Verwijderen</button></div>
    ${renderDetails(selected)}
  </section>${objectDialogMode === 'object' ? renderObjectFormDialog(selected, type) : ''}${objectDialogMode === 'value' ? renderValueDialog() : ''}${objectDialogMode === 'relation' ? renderRelationDialog(selected) : ''}`;
}

function renderObjectFormDialog(selected, type) {
  return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="object-dialog-heading">
    <div class="core-heading"><h2 id="object-dialog-heading">${selected ? `${types[type]} wijzigen` : objectSections[type].create}</h2><button type="button" class="secondary" data-action="close-object-dialog">Sluiten</button></div>
    <form id="object-form">
      <label>Type <select name="objectType">${Object.entries(types).map(([value, label]) => `<option value="${value}" ${type === value ? 'selected' : ''}>${label}</option>`).join('')}</select></label>
      <label>Naam <input name="name" value="${escapeHtml(selected?.name)}" required /></label>
      <label>Omschrijving <textarea name="description" rows="3">${escapeHtml(selected?.description)}</textarea></label>
      <div class="core-actions"><button type="submit">${selected ? 'Opslaan' : 'Aanmaken'}</button></div>
    </form>
  </section></div>`;
}

function focusObjectType(type) {
  objectTypeFocus = type;
  render();
  if (type !== 'all') document.querySelector(`#${objectSections[type].id}`)?.scrollIntoView({ block: 'nearest' });
}

function dashboardPeriodLabel() {
  if (!dashboardMonth) return `${dashboardYear} · hele jaar`;
  return `${new Intl.DateTimeFormat('nl-NL', { month: 'long' }).format(new Date(dashboardYear, dashboardMonth - 1, 1))} ${dashboardYear}`;
}

function dashboardDate(value) {
  return new Intl.DateTimeFormat('nl-NL', { day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(`${value}T00:00:00`));
}

function renderDashboard() {
  const currentYear = new Date().getFullYear();
  const years = [...new Set([currentYear - 1, currentYear, currentYear + 1, dashboardYear, ...(dashboard?.availableYears || [])])]
    .filter((year) => year >= 1 && year <= 9999).sort((left, right) => right - left);
  const ready = !dashboardLoading && !dashboardError && dashboard;
  return `<section class="dashboard-period-panel" aria-label="Dashboardperiode">
    <div class="period-filter"><span class="period-filter-label">Jaar</span>
      <div class="period-buttons" role="group" aria-label="Dashboardjaar">${years.map((year) => `<button type="button" class="period-button ${year === dashboardYear ? 'active' : ''}" data-action="dashboard-year" data-year="${year}" aria-pressed="${year === dashboardYear}">${year}</button>`).join('')}</div>
    </div>
    <div class="period-filter"><span class="period-filter-label">Maand</span>
      <div class="period-buttons month-buttons" role="group" aria-label="Dashboardmaand"><button type="button" class="period-button ${dashboardMonth === 0 ? 'active' : ''}" data-action="dashboard-month" data-month="0" aria-pressed="${dashboardMonth === 0}">Hele jaar</button>${dashboardMonths.map((label, index) => `<button type="button" class="period-button ${dashboardMonth === index + 1 ? 'active' : ''}" data-action="dashboard-month" data-month="${index + 1}" aria-pressed="${dashboardMonth === index + 1}">${label}</button>`).join('')}</div>
    </div>
    <p class="dashboard-period-label" role="status">${escapeHtml(dashboardPeriodLabel())}${dashboardLoading ? ' · gegevens laden…' : ''}</p>
  </section>
  ${dashboardError ? `<p class="error" role="alert">${escapeHtml(dashboardError)}</p>` : ''}
  <div class="dashboard-grid" aria-busy="${dashboardLoading}">
    <details id="dashboard-balance-frame" class="dashboard-panel" open>
      <summary><h2>Balans &amp; Planning</h2><strong>${ready ? money.format(dashboard.netWorthCents / 100) : '—'}</strong></summary>
      ${ready ? renderDashboardBalance() : `<p class="muted">${dashboardError ? 'Waarderingen konden niet worden geladen.' : 'Waarderingen voor de gekozen peildatum worden geladen.'}</p>`}
    </details>
    <details id="dashboard-transactions-frame" class="dashboard-panel" open>
      <summary><h2>Transacties</h2><strong>${ready ? `${dashboard.transactions.count} posten` : '—'}</strong></summary>
      ${ready ? renderDashboardTransactions() : `<p class="muted">${dashboardError ? 'Transacties konden niet worden geladen.' : 'Transacties voor de gekozen periode worden geladen.'}</p>`}
    </details>
  </div>`;
}

function renderDashboardBalance() {
  const group = (kind, label) => {
    const item = dashboard.objectGroups.find((entry) => entry.objectType === kind);
    const value = item.count === 0 ? 'Geen objecten' : (item.valuedCount ? money.format(item.valueCents / 100) : 'Geen waardering');
    return `<div class="dashboard-wealth-card"><span>${label}<small>${item.count} objecten · ${item.valuedCount} met waarde</small></span>
      <span class="dashboard-wealth-values"><strong>${value}</strong><small>Gekoppelde mutaties: ${money.format(item.periodAmountCents / 100)}</small></span></div>`;
  };
  const missing = dashboard.objectGroups.reduce((count, item) => count + item.count - item.valuedCount, 0);
  return `<p class="dashboard-caption">Peildatum: ${escapeHtml(dashboardDate(dashboard.to))}</p>
    <div class="dashboard-wealth-summary">
      ${group('asset', 'Bezittingen')}${group('debt', 'Schulden')}
      <div class="dashboard-wealth-card dashboard-wealth-net"><span>Netto balans<small>Bezittingen − schulden</small></span><span class="dashboard-wealth-values"><strong>${money.format(dashboard.netWorthCents / 100)}</strong></span></div>
      ${group('reserve', 'Reserveringen')}${group('expense', 'Lasten')}
    </div>
    <p class="dashboard-caption">Op basis van de laatste vastgelegde waarden tot de peildatum. Reserveringen en lasten worden apart getoond.</p>
    ${missing ? `<p class="dashboard-caption">${missing} objecten zonder waardering op deze peildatum.</p>` : ''}
    <a class="dashboard-open-link" href="#balans-planning">Open Balans &amp; Planning →</a>`;
}

function renderDashboardTransactions() {
  const item = dashboard.transactions;
  const maximum = Math.max(item.incomeCents, -item.expenseCents, 1);
  return `<p class="dashboard-caption">${escapeHtml(dashboardDate(dashboard.from))} t/m ${escapeHtml(dashboardDate(dashboard.to))}</p>
    <div class="dashboard-wealth-summary">
      <div class="dashboard-wealth-card"><span>Inkomsten<small>${item.incomeCount} transacties</small></span><span class="dashboard-wealth-values"><strong class="income">${money.format(item.incomeCents / 100)}</strong></span></div>
      <div class="dashboard-wealth-card"><span>Uitgaven<small>${item.expenseCount} transacties</small></span><span class="dashboard-wealth-values"><strong class="expense">${money.format(-item.expenseCents / 100)}</strong></span></div>
      <div class="dashboard-wealth-card dashboard-wealth-net"><span>Saldo van de periode</span><span class="dashboard-wealth-values"><strong>${money.format(item.balanceCents / 100)}</strong></span></div>
    </div>
    <div class="dashboard-cashflow-chart" role="img" aria-label="Vergelijking van inkomsten en uitgaven in de geselecteerde periode">
      <div><span>Inkomsten</span><div class="dashboard-bar-track"><div class="dashboard-bar income-bar" style="width:${item.incomeCents / maximum * 100}%"></div></div></div>
      <div><span>Uitgaven</span><div class="dashboard-bar-track"><div class="dashboard-bar expense-bar" style="width:${-item.expenseCents / maximum * 100}%"></div></div></div>
    </div>
    <p class="dashboard-caption">${item.count === 0 ? 'Geen transacties in deze periode.' : (item.unallocatedCount ? `${item.unallocatedCount} transacties nog niet volledig toegewezen.` : 'Alle transacties zijn volledig toegewezen.')}</p>
    <a class="dashboard-open-link" href="#inkomsten-uitgaven">Open Inkomsten &amp; Uitgaven →</a>`;
}

function updateDashboard() {
  const root = document.querySelector('#dashboard-content');
  if (!root) return;
  const focused = root.contains(document.activeElement) ? document.activeElement.dataset : null;
  const focusSelector = focused?.action === 'dashboard-year' ? `[data-action="dashboard-year"][data-year="${focused.year}"]`
    : (focused?.action === 'dashboard-month' ? `[data-action="dashboard-month"][data-month="${focused.month}"]` : null);
  const openPanels = [...root.querySelectorAll('details[id]')].map((panel) => [panel.id, panel.open]);
  root.innerHTML = renderDashboard();
  for (const [id, open] of openPanels) {
    const panel = root.querySelector(`#${id}`);
    if (panel) panel.open = open;
  }
  if (focusSelector) root.querySelector(focusSelector)?.focus({ preventScroll: true });
}

async function loadDashboard() {
  const currentRequest = ++dashboardRequestID;
  const params = new URLSearchParams({ year: dashboardYear, month: dashboardMonth });
  dashboardLoading = true;
  dashboardError = '';
  updateDashboard();
  try {
    const result = await api(`/dashboard?${params}`);
    if (currentRequest !== dashboardRequestID) return;
    dashboard = result;
    dashboardLoading = false;
    updateDashboard();
  } catch (error) {
    if (currentRequest !== dashboardRequestID) return;
    dashboardLoading = false;
    dashboardError = error.message;
    updateDashboard();
  }
}

function activateTab() {
  const requested = window.location.hash.slice(1);
  activeTab = Object.hasOwn(tabs, requested) ? requested : 'balans-planning';
  document.title = `${tabs[activeTab]} · HoHoHo`;
  for (const section of app.querySelectorAll('.core-tab-section')) {
    section.hidden = section.id !== activeTab;
  }
  for (const link of app.querySelectorAll('.core-navigation [data-tab]')) {
    if (link.dataset.tab === activeTab) link.setAttribute('aria-current', 'page');
    else link.removeAttribute('aria-current');
  }
  if (activeTab === 'import') updateBankImport();
}

function renderDetails(selected) {
  const objectName = (id) => objects.find((object) => object.id === id)?.name || `Object ${id}`;
  const linkedDocuments = documents.filter((document) => (document.financialObjectIds || []).includes(selected.id));
  const availableDocuments = documents.filter((document) => !(document.financialObjectIds || []).includes(selected.id));
  return `<section class="core-detail"><h3>Waardehistorie</h3>
    ${values.length ? `<ul class="core-detail-list">${values.map((value) => `<li><span>${escapeHtml(value.valueDate)} · ${money.format(value.valueCents / 100)}</span><button type="button" class="secondary" data-action="delete-value" data-id="${value.id}">Verwijderen</button></li>`).join('')}</ul>` : '<p class="muted">Nog geen waarderingen.</p>'}
  </section>
  <section class="core-detail"><h3>Relaties</h3>
    ${relations.length ? `<ul class="core-detail-list">${relations.map((relation, index) => `<li><span>${escapeHtml(objectName(relation.sourceObjectId))} → <strong>${escapeHtml(relation.relationType)}</strong> → ${escapeHtml(objectName(relation.targetObjectId))}</span><button type="button" class="secondary" data-action="delete-relation" data-index="${index}">Ontkoppelen</button></li>`).join('')}</ul>` : '<p class="muted">Nog geen inkomende of uitgaande relaties.</p>'}
  </section>
  <section class="core-detail"><div class="core-heading"><h3>Bijbehorende transacties</h3><a class="dashboard-open-link" href="#inkomsten-uitgaven">Open Inkomsten &amp; Uitgaven</a></div>
    ${objectTransactions.length ? `<div class="core-table-scroll"><table class="core-transaction-table"><thead><tr><th>Datum</th><th>Omschrijving / tegenpartij</th><th>Bedrag</th><th>Toewijzing</th></tr></thead><tbody>${objectTransactions.map((item) => {
      const link = item.allocations.find((allocation) => allocation.financialObjectId === selected.id);
      return `<tr><td>${escapeHtml(item.transactionDate)}</td><td><strong>${escapeHtml(item.description || 'Geen omschrijving')}</strong><small>${escapeHtml(item.counterparty)}</small></td><td class="${item.amountCents > 0 ? 'income' : 'expense'}">${money.format(item.amountCents / 100)}</td><td>${link?.amountCents == null ? 'Context' : money.format(link.amountCents / 100)}</td></tr>`;
    }).join('')}</tbody></table></div>` : '<p class="muted">Nog geen transacties gekoppeld aan dit object.</p>'}
  </section>
  <section class="core-detail"><h3>Documenten</h3>
    ${availableDocuments.length ? `<form id="object-document-link-form" class="core-inline-form"><label>Document <select name="documentId">${availableDocuments.map((document) => `<option value="${document.id}">${escapeHtml(document.filename)} · ${escapeHtml(documentStatuses[document.status] || document.status || 'Onbekend')}</option>`).join('')}</select></label><button type="submit">Document koppelen</button></form>` : '<p class="muted">Geen ongekoppelde documenten beschikbaar.</p>'}
    ${linkedDocuments.length ? `<ul class="core-detail-list">${linkedDocuments.map((document) => `<li><span>${escapeHtml(document.filename)} · ${escapeHtml(documentStatuses[document.status] || document.status || 'Onbekend')}</span><div class="core-actions"><button type="button" class="secondary" data-action="open-document" data-id="${document.id}">Openen</button><button type="button" class="secondary" data-action="unlink-object-document" data-id="${document.id}">Ontkoppelen</button></div></li>`).join('')}</ul>` : '<p class="muted">Nog geen documenten gekoppeld.</p>'}
  </section>`;
}

function renderValueDialog() {
  const today = new Date().toLocaleDateString('sv-SE');
  return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="value-dialog-heading">
    <div class="core-heading"><h2 id="value-dialog-heading">Waarde toevoegen</h2><button type="button" class="secondary" data-action="close-object-dialog">Sluiten</button></div>
    <form id="value-form" class="core-inline-form">
      <label>Waardedatum <input name="valueDate" type="date" value="${today}" required /></label>
      <label>Waarde (€) <input name="amount" type="text" inputmode="decimal" placeholder="598500,00" required /></label>
      <button type="submit">Toevoegen</button>
    </form>
  </section></div>`;
}

function renderRelationDialog(selected) {
  const targets = objects.filter((object) => object.id !== selected.id);
  return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="relation-dialog-heading">
    <div class="core-heading"><h2 id="relation-dialog-heading">Relatie toevoegen</h2><button type="button" class="secondary" data-action="close-object-dialog">Sluiten</button></div>
    ${targets.length ? `<form id="relation-form">
      <label>Relatietype <input name="relationType" placeholder="reserved_for" required /></label>
      <label>Naar <select name="targetObjectId">${targets.map((object) => `<option value="${object.id}">${escapeHtml(object.name)} (${types[object.objectType]})</option>`).join('')}</select></label>
      <button type="submit">Toevoegen</button>
    </form>` : '<p class="muted">Maak eerst een tweede object aan.</p>'}
  </section></div>`;
}

function amountInput(value) {
  return value == null ? '' : (Math.abs(value) / 100).toFixed(2).replace('.', ',');
}

function renderAllocationRow(link = {}) {
  return `<div class="allocation-row">
    <label>Financieel object <select class="allocation-object"><option value="">Kies een object</option>${objects.map((object) => `<option value="${object.id}" ${object.id === link.financialObjectId ? 'selected' : ''}>${escapeHtml(object.name)} (${types[object.objectType]})</option>`).join('')}</select></label>
    <label>Toegewezen bedrag (€) <input class="allocation-amount" type="text" inputmode="decimal" value="${amountInput(link.amountCents)}" placeholder="Leeg = alleen koppelen" /></label>
    <div class="core-actions"><button type="button" class="secondary" data-action="allocate-whole">Hele bedrag</button><button type="button" class="secondary" data-action="remove-allocation">Koppeling weghalen</button></div>
  </div>`;
}

function renderTransactions() {
  const draft = transactionDraft;
  const direction = draft ? (draft.amountCents > 0 ? 'income' : 'expense') : (transactionViewFilter === 'income' ? 'income' : 'expense');
  const links = draft?.allocations || (selectedID == null ? [] : [{ financialObjectId: selectedID }]);
  const rows = links.length ? links : (objects.length ? [{}] : []);
  return `<section class="core-panel core-transactions" aria-labelledby="transactions-heading">
    <div class="core-heading"><h2 id="transactions-heading">Transacties</h2><div class="core-actions"><button type="button" data-action="classify-transactions-ai">Classificeren met AI</button><button type="button" data-action="new-transaction">Nieuwe transactie</button></div></div>
    <p class="muted">Leg een ontvangst of betaling vast en koppel deze aan je financiële objecten. Een koppeling zonder bedrag is alleen context.</p>
    <form id="transaction-filter-form" class="transaction-filter-bar">
      <label>Toon <select name="view"><option value="">Zichtbare transacties</option><option value="income" ${transactionViewFilter === 'income' ? 'selected' : ''}>Inkomsten</option><option value="expense" ${transactionViewFilter === 'expense' ? 'selected' : ''}>Uitgaven</option><option value="hidden" ${transactionViewFilter === 'hidden' ? 'selected' : ''}>Verborgen transacties</option></select></label>
      <label>Vanaf <input name="from" type="date" value="${escapeHtml(transactionFromFilter)}" /></label>
      <label>Tot en met <input name="to" type="date" value="${escapeHtml(transactionToFilter)}" /></label>
      <label>Zoeken <input name="search" value="${escapeHtml(transactionSearchFilter)}" placeholder="Omschrijving of tegenpartij" /></label>
      <div class="core-actions"><button type="submit">Filter toepassen</button><button type="button" class="secondary" data-action="reset-transaction-filters">Reset</button></div>
    </form>
    <div class="core-stats" aria-label="Totalen van de geselecteerde transacties">
      <div><strong>${money.format(transactionTotals.incomeCents / 100)}</strong><span>Inkomsten</span></div>
      <div><strong>${money.format(-transactionTotals.expenseCents / 100)}</strong><span>Uitgaven</span></div>
      <div><strong>${money.format(transactionTotals.balanceCents / 100)}</strong><span>Saldo van de selectie</span></div>
    </div>
    ${transactions.length ? `<div class="core-table-scroll"><table class="core-transaction-table"><thead><tr><th>${renderSortButton('date', 'Datum')}</th><th>${renderSortButton('description', 'Omschrijving / tegenpartij')}</th><th>${renderSortButton('amount', 'Bedrag')}</th><th>Status</th><th>Objecten / toewijzing</th><th>Acties</th></tr></thead><tbody>${transactions.map((item) => `<tr>
      <td>${escapeHtml(item.transactionDate)}</td>
      <td><strong>${escapeHtml(item.description || 'Geen omschrijving')}</strong><small>${escapeHtml(item.counterparty)}</small></td>
      <td class="${item.amountCents > 0 ? 'income' : 'expense'}">${money.format(item.amountCents / 100)}<small>${item.amountCents > 0 ? 'Inkomst' : 'Uitgave'}</small></td>
      <td><span class="config-status ${item.status === 'A' || item.status === 'U' ? 'ok' : 'warning'}">${escapeHtml(transactionStatuses[item.status] || item.status || 'Onbekend')}</span></td>
      <td>${item.hidden ? '<span class="muted">Verborgen</span>' : (item.allocations.length ? `<ul class="allocation-labels">${item.allocations.map((link) => `<li>${escapeHtml(link.name)} · ${link.amountCents == null ? 'context' : money.format(link.amountCents / 100)}</li>`).join('')}</ul>` : '<span class="muted">Nog niet geclassificeerd</span>')}<small>${item.fullyAllocated ? 'Volledig toegewezen' : `Nog toe te wijzen: ${money.format(item.unallocatedAmountCents / 100)}`}</small></td>
      <td><div class="core-actions"><button type="button" class="secondary" data-action="edit-transaction" data-id="${item.id}" ${item.hidden ? 'disabled' : ''}>Wijzigen</button>${item.hidden ? `<button type="button" class="secondary" data-action="unhide-transaction" data-id="${item.id}">Zichtbaar maken</button>` : `<button type="button" class="secondary" data-action="delete-transaction" data-id="${item.id}">Verbergen</button>`}</div></td>
    </tr>`).join('')}</tbody></table></div>` : '<p class="muted">Nog geen transacties in deze selectie.</p>'}
    <div class="core-pagination"><button type="button" class="secondary" data-action="previous-transactions" ${transactionOffset === 0 ? 'disabled' : ''}>Vorige</button><span>${transactionTotals.total ? `${transactionOffset + 1}–${Math.min(transactionOffset + transactions.length, transactionTotals.total)} van ${transactionTotals.total}` : '0 transacties'}</span><button type="button" class="secondary" data-action="next-transactions" ${transactionOffset + transactionPageSize >= transactionTotals.total ? 'disabled' : ''}>Volgende</button></div>
    <section class="core-detail"><h3>${draft ? 'Transactie wijzigen' : 'Transactie aanmaken'}</h3>
      <form id="transaction-form">
        <div class="transaction-fields">
          <label>Soort <select name="direction"><option value="expense" ${direction === 'expense' ? 'selected' : ''}>Uitgave</option><option value="income" ${direction === 'income' ? 'selected' : ''}>Inkomst</option></select></label>
          <label>Datum <input name="transactionDate" type="date" value="${escapeHtml(draft?.transactionDate || new Date().toLocaleDateString('sv-SE'))}" required /></label>
          <label>Bedrag (€) <input name="amount" type="text" inputmode="decimal" value="${amountInput(draft?.amountCents)}" placeholder="83,42" required /></label>
          <label>Tegenpartij <input name="counterparty" value="${escapeHtml(draft?.counterparty)}" /></label>
        </div>
        <label>Omschrijving <input name="description" value="${escapeHtml(draft?.description)}" /></label>
        <p class="muted">Vul bedragen positief in; de gekozen soort bepaalt het teken. Splits een bedrag over meerdere objecten of laat het nog ongeclassificeerd. Waardehistorie wordt afzonderlijk onderhouden.</p>
        <div id="allocation-rows">${rows.map(renderAllocationRow).join('')}</div>
        <button type="button" class="secondary" data-action="add-allocation" ${objects.length ? '' : 'disabled'}>Object koppelen / splitsing toevoegen</button>
        <label><input name="applyToMatching" type="checkbox" checked /> Zelfde categorisatie toepassen op zichtbare transacties met dezelfde tegenpartij die nog niet geclassificeerd zijn</label>
        <div class="core-actions"><button type="submit">${draft ? 'Transactie opslaan' : 'Transactie aanmaken'}</button>${draft ? '<button type="button" class="secondary" data-action="new-transaction">Annuleren</button>' : ''}</div>
      </form>
    </section>
  </section>`;
}

function renderSortButton(sort, label) {
  const active = transactionSort === sort;
  const nextOrder = active && transactionOrder === 'asc' ? 'desc' : 'asc';
  const direction = !active ? '' : (transactionOrder === 'asc' ? ' ↑' : ' ↓');
  return `<button type="button" class="table-sort-button ${active ? 'active' : ''}" data-action="sort-transactions" data-sort="${sort}" data-order="${nextOrder}" aria-sort="${active ? (transactionOrder === 'asc' ? 'ascending' : 'descending') : 'none'}">${escapeHtml(label)}${direction}</button>`;
}

function parseCents(value) {
  const match = String(value).trim().match(/^(-?)(\d+)(?:[.,](\d{1,2}))?$/);
  if (!match) throw new Error('Vul een bedrag in met maximaal twee decimalen, bijvoorbeeld 1234,56.');
  const cents = (BigInt(match[2]) * 100n + BigInt((match[3] || '').padEnd(2, '0'))) * (match[1] ? -1n : 1n);
  if (cents > BigInt(Number.MAX_SAFE_INTEGER) || cents < BigInt(Number.MIN_SAFE_INTEGER)) throw new Error('Dit bedrag is te groot voor invoer via de browser.');
  return Number(cents);
}

function positiveCents(value) {
  const amount = parseCents(value);
  if (amount <= 0) throw new Error('Vul een positief bedrag groter dan nul in; kies Inkomst of Uitgave voor de richting.');
  return amount;
}

function showError(error) {
  message = error.message;
  const element = document.querySelector('#message');
  if (element) {
    element.textContent = message;
    element.scrollIntoView({ block: 'nearest' });
  } else app.textContent = message;
}

async function mutate(button, action) {
  button.disabled = true;
  message = '';
  try {
    await action();
    await refresh();
  } catch (error) {
    showError(error);
  } finally {
    button.disabled = false;
  }
}

app.addEventListener('change', (event) => {
  if (event.target.closest('#transaction-filter-form') && event.target.name === 'view') {
    transactionViewFilter = event.target.value;
    transactionOffset = 0;
    refresh().catch(showError);
  }
  if (event.target.closest('#application-config-form') && event.target.name === 'transactionAiUrl') {
    loadTransactionAIModels(event.target.value).catch(showError);
  }
  if (event.target.closest('#bank-import-form')) {
    const form = event.target.closest('#bank-import-form');
    if (event.target.name === 'bankConfig') {
      bankImportSelectedConfig = event.target.value;
      bankImportPreview = null;
      bankImportMessage = '';
      updateBankImport();
    }
    if (event.target.name === 'file') {
      bankImportFile = form.elements.file.files[0] || null;
      bankImportPreview = null;
      bankImportMessage = bankImportFile ? 'Bestand geselecteerd. Je kunt direct importeren of eerst een preview maken.' : '';
      updateBankImport();
    }
  }
});

app.addEventListener('input', (event) => {
  if (event.target.closest('#application-config-form') && event.target.name === 'transactionAiUrl') {
    transactionAIModels = [];
    transactionAIModelsError = '';
    updateTransactionAIModelControls();
  }
});

app.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-action]');
  if (!button || button.disabled) return;
  const action = button.dataset.action;
  const objectID = selectedID;
  if (action === 'object-type') {
    focusObjectType(button.dataset.type);
    return;
  }
  if (action === 'close-object-editor') {
    objectEditorOpen = false;
    objectDialogMode = '';
    selectedID = null;
    ++requestID;
    render();
    return;
  }
  if (action === 'open-object-form' || action === 'open-value-form' || action === 'open-relation-form') {
    objectDialogMode = action === 'open-object-form' ? 'object' : (action === 'open-value-form' ? 'value' : 'relation');
    render();
    return;
  }
  if (action === 'close-object-dialog') {
    if (selectedID == null) objectEditorOpen = false;
    objectDialogMode = '';
    render();
    return;
  }
  if (action === 'dashboard-year') {
    dashboardYear = Number(button.dataset.year);
    dashboardMonth = 0;
    await loadDashboard();
    return;
  }
  if (action === 'dashboard-month') {
    const month = Number(button.dataset.month);
    dashboardMonth = dashboardMonth === month ? 0 : month;
    await loadDashboard();
    return;
  }
  if (action === 'configuration-section') {
    configurationFocus = button.dataset.section;
    render();
    return;
  }
  if (action === 'load-transaction-ai-models') {
    const form = document.querySelector('#application-config-form');
    const url = form?.elements?.transactionAiUrl?.value || '';
    await loadTransactionAIModels(url);
    return;
  }
  if (action === 'new-bank-config') {
    try {
      const slug = `bank-${bankConfigs.length + 1}`;
      const template = await api(`/bank-configs/template?slug=${encodeURIComponent(slug)}`);
      selectedBankConfig = { slug, rawYaml: template.rawYaml, status: { valid: true } };
      bankConfigMessage = '';
      render();
    } catch (error) { showError(error); }
    return;
  }
  if (action === 'select-bank-config') {
    try {
      selectedBankConfig = await api(`/bank-configs/${encodeURIComponent(button.dataset.slug)}`);
      bankConfigMessage = '';
      render();
    } catch (error) { showError(error); }
    return;
  }
  if (action === 'delete-bank-config') {
    await mutate(button, async () => {
      await api(`/bank-configs/${encodeURIComponent(button.dataset.slug)}`, { method: 'DELETE' });
      bankConfigs = await api('/bank-configs');
      selectedBankConfig = bankConfigs[0] || null;
      bankConfigMessage = 'Bankconfiguratie is verwijderd.';
      render();
    });
    return;
  }
  if (action === 'classify-transactions-ai') {
    await mutate(button, async () => {
      const result = await api('/transactions/classify-ai', { method: 'POST', body: JSON.stringify({}) });
      message = `${result.classified || 0} transacties door AI geclassificeerd; ${result.failed || 0} mislukt.`;
      transactionOffset = 0;
    });
    return;
  }
  if (action === 'import-bank-transactions') {
    const form = document.querySelector('#bank-import-form');
    if (!form) return;
    const file = bankImportFile || form.elements.file.files[0];
    const bankConfig = form.elements.bankConfig.value || bankImportSelectedConfig || bankImportPreview?.bankConfig;
    if (!file || !bankConfig) { showError(new Error('Kies een bankconfiguratie en bankbestand.')); return; }
    button.disabled = true;
    message = '';
    try {
      const data = new FormData();
      data.set('bankConfig', bankConfig);
      data.set('file', file);
      const response = await fetch(`${apiBaseUrl}/api/v1/bank-imports`, { method: 'POST', body: data });
      if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || `Import mislukt (${response.status})`);
      const result = await response.json();
      bankImportMessage = `${result.imported} transacties geïmporteerd.`;
      bankImportPreview = null;
      bankImportFile = null;
      bankImportSelectedConfig = bankConfig;
      transactionOffset = 0;
      await refresh();
      activeTab = 'import';
      activateTab();
    } catch (error) { showError(error); } finally { button.disabled = false; }
    return;
  }
  if (action === 'add-allocation') {
    document.querySelector('#allocation-rows').insertAdjacentHTML('beforeend', renderAllocationRow());
    return;
  }
  if (action === 'reset-transaction-filters') {
    transactionViewFilter = '';
    transactionFromFilter = '';
    transactionToFilter = '';
    transactionSearchFilter = '';
    transactionOffset = 0;
    try { await refresh(); } catch (error) { showError(error); }
    return;
  }
  if (action === 'sort-transactions') {
    transactionSort = button.dataset.sort;
    transactionOrder = button.dataset.order;
    transactionOffset = 0;
    try { await refresh(); } catch (error) { showError(error); }
    return;
  }
  if (action === 'remove-allocation') {
    button.closest('.allocation-row').remove();
    return;
  }
  if (action === 'allocate-whole') {
    try {
      const amount = button.closest('form').elements.amount.value;
      positiveCents(amount);
      button.closest('.allocation-row').querySelector('.allocation-amount').value = amount;
    } catch (error) { showError(error); }
    return;
  }
  if (action === 'new-transaction') {
    ++transactionEditRequestID;
    transactionDraft = null;
    message = '';
    render();
    document.querySelector('#transaction-form').scrollIntoView({ block: 'center' });
    return;
  }
  if (action === 'edit-transaction') {
    const editRequest = ++transactionEditRequestID;
    try {
      const draft = await api(`/transactions/${button.dataset.id}`);
      if (editRequest !== transactionEditRequestID) return;
      transactionDraft = draft;
      message = '';
      render();
      document.querySelector('#transaction-form').scrollIntoView({ block: 'center' });
    } catch (error) { showError(error); }
    return;
  }
  if (action === 'previous-transactions' || action === 'next-transactions') {
    transactionOffset = Math.max(0, transactionOffset + (action === 'next-transactions' ? transactionPageSize : -transactionPageSize));
    try { await refresh(); } catch (error) { showError(error); }
    return;
  }
  if (action === 'backup-validate') {
    await mutate(button, async () => {
      backupStatus = await api('/system/backup/validate', { method: 'POST' });
      backupMessage = 'Backupconfiguratie is gecontroleerd.';
      render();
    });
    return;
  }
  if (action === 'backup-run') {
    await mutate(button, async () => {
      backupStatus = await api('/system/backup/runs', { method: 'POST' });
      backupMessage = 'Backup is gestart.';
      render();
    });
    return;
  }
  if (action === 'new-document') {
    documentUploadOpen = true;
    updateDocuments();
    return;
  }
  if (action === 'document-help') {
    documentHelpOpen = true;
    updateDocuments();
    return;
  }
  if (action === 'close-document-upload') {
    documentUploadOpen = false;
    updateDocuments();
    return;
  }
  if (action === 'close-document-help') {
    documentHelpOpen = false;
    updateDocuments();
    return;
  }
  if (action === 'refresh-documents') {
    await mutate(button, reloadDocuments);
    return;
  }
  if (action === 'open-document') {
    selectedDocumentID = Number(button.dataset.id);
    activeTab = 'documenten';
    render();
    activateTab();
    document.querySelector('.document-viewer')?.scrollIntoView({ block: 'start' });
    return;
  }
  if (action === 'close-document-viewer') {
    selectedDocumentID = null;
    updateDocuments();
    return;
  }
  if (action === 'backup-snapshots') {
    await mutate(button, async () => {
      backupSnapshots = await api('/system/backup/snapshots');
      backupMessage = 'Snapshots zijn ververst.';
      render();
    });
    return;
  }
  if (action === 'select-recovery-snapshot') {
    const form = document.querySelector('#recovery-form');
    if (form) form.elements.snapshotId.value = button.dataset.id;
    return;
  }
  if (action === 'select' || action === 'new') {
    selectedID = action === 'select' ? Number(button.dataset.id) : null;
    objectTypeFocus = action === 'new' ? button.dataset.type : objects.find((object) => object.id === selectedID).objectType;
    objectEditorOpen = true;
    objectDialogMode = action === 'new' ? 'object' : '';
    message = '';
    if (action === 'new') {
      values = [];
      relations = [];
      objectTransactions = [];
      render();
      document.querySelector('#object-form')?.scrollIntoView({ block: 'center' });
      return;
    }
    await loadSelectedObjectDetails(selectedID);
    document.querySelector('#object-editor')?.scrollIntoView({ block: 'start' });
    return;
  }
  await mutate(button, async () => {
    if (action === 'delete-object') {
      await api(`/financial-objects/${button.dataset.id}`, { method: 'DELETE' });
      selectedID = null;
      objectEditorOpen = false;
      objectDialogMode = '';
    } else if (action === 'delete-value') {
      await api(`/financial-objects/${objectID}/values/${button.dataset.id}`, { method: 'DELETE' });
    } else if (action === 'delete-relation') {
      const relation = relations[Number(button.dataset.index)];
      await api('/object-relations', { method: 'DELETE', body: JSON.stringify(relation) });
    } else if (action === 'unlink-object-document') {
      await api(`/documents/${button.dataset.id}/links/${objectID}`, { method: 'DELETE' });
    } else if (action === 'delete-transaction') {
      const id = Number(button.dataset.id);
      await api(`/transactions/${id}`, { method: 'DELETE' });
      if (transactionDraft?.id === id) transactionDraft = null;
      if (transactions.length === 1 && transactionOffset > 0) transactionOffset -= transactionPageSize;
    } else if (action === 'unhide-transaction') {
      await api(`/transactions/${button.dataset.id}/unhide`, { method: 'POST' });
      if (transactions.length === 1 && transactionOffset > 0) transactionOffset -= transactionPageSize;
    } else if (action === 'analyze-document') {
      await api(`/documents/${button.dataset.id}/analyze`, { method: 'POST' });
      selectedDocumentID = Number(button.dataset.id);
      documentMessage = 'Documentanalyse is uitgevoerd.';
      await reloadDocuments();
    } else if (action === 'manual-document') {
      await api(`/documents/${button.dataset.id}/classify/manual`, { method: 'POST' });
      documentMessage = 'Document handmatig geclassificeerd.';
      await reloadDocuments();
    } else if (action === 'delete-document') {
      await api(`/documents/${button.dataset.id}`, { method: 'DELETE' });
      documentMessage = 'Document verwijderd.';
      await reloadDocuments();
    }
  });
});

app.addEventListener('submit', async (event) => {
  event.preventDefault();
  const form = event.target;
  const button = form.querySelector('button[type="submit"]');
  if (!button || button.disabled) return;
  if (form.id === 'document-upload-form') {
    const file = form.elements.file.files[0];
    if (!file) { showError(new Error('Kies een bestand.')); return; }
    button.disabled = true;
    message = '';
    try {
      const data = new FormData();
      data.set('file', file);
      const response = await fetch(`${apiBaseUrl}/api/v1/documents`, { method: 'POST', body: data });
      if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || `Upload mislukt (${response.status})`);
      await response.json();
      form.reset();
      documentUploadOpen = false;
      documentMessage = 'Document geüpload.';
      await reloadDocuments();
    } catch (error) { showError(error); } finally { button.disabled = false; }
    return;
  }
  const data = Object.fromEntries(new FormData(form));
  const objectID = selectedID;
  await mutate(button, async () => {
    if (form.id === 'object-form') {
      const object = await api(objectID == null ? '/financial-objects' : `/financial-objects/${objectID}`, {
        method: objectID == null ? 'POST' : 'PUT', body: JSON.stringify(data),
      });
      selectedID = object.id;
      objectTypeFocus = object.objectType;
      objectEditorOpen = true;
      objectDialogMode = '';
    } else if (form.id === 'value-form') {
      await api(`/financial-objects/${objectID}/values`, {
        method: 'POST', body: JSON.stringify({ valueDate: data.valueDate, valueCents: parseCents(data.amount) }),
      });
      objectDialogMode = '';
    } else if (form.id === 'relation-form') {
      await api('/object-relations', {
        method: 'POST', body: JSON.stringify({ sourceObjectId: objectID, targetObjectId: Number(data.targetObjectId), relationType: data.relationType }),
      });
      objectDialogMode = '';
    } else if (form.id === 'object-document-link-form') {
      await api(`/documents/${data.documentId}/links`, {
        method: 'POST', body: JSON.stringify({ financialObjectId: objectID }),
      });
    } else if (form.id === 'transaction-form') {
      const sign = data.direction === 'income' ? 1 : -1;
      const allocations = [];
      for (const row of form.querySelectorAll('.allocation-row')) {
        const object = row.querySelector('.allocation-object').value;
        const amount = row.querySelector('.allocation-amount').value.trim();
        if (!object) {
          if (amount) throw new Error('Kies een financieel object bij elk toegewezen bedrag.');
          continue;
        }
        allocations.push({ financialObjectId: Number(object), amountCents: amount ? sign * positiveCents(amount) : null });
      }
      await api(transactionDraft ? `/transactions/${transactionDraft.id}` : '/transactions', {
        method: transactionDraft ? 'PUT' : 'POST',
        body: JSON.stringify({ transactionDate: data.transactionDate, amountCents: sign * positiveCents(data.amount), description: data.description, counterparty: data.counterparty, allocations, applyToMatching: form.elements.applyToMatching.checked }),
      });
      ++transactionEditRequestID;
      transactionDraft = null;
      transactionOffset = 0;
    } else if (form.id === 'configuration-form') {
      const accessKey = data.s3AccessKey.trim();
      const secretKey = data.s3SecretKey.trim();
      const payload = {
        s3Endpoint: data.s3Endpoint.trim(), s3Bucket: data.s3Bucket.trim(), s3Region: data.s3Region.trim(), pdfAiUrl: configuration.runtime.pdfAiUrl || '', transactionAiUrl: configuration.runtime.transactionAiUrl || '', transactionAiModel: configuration.runtime.transactionAiModel || transactionAIModels[0] || defaultTransactionAIModel,
        s3UseTLS: form.elements.s3UseTLS.checked, s3PathStyle: form.elements.s3PathStyle.checked, s3CreateBucket: form.elements.s3CreateBucket.checked,
      };
      if (accessKey) payload.s3AccessKey = accessKey;
      if (secretKey) payload.s3SecretKey = secretKey;
      configuration = await api('/system/configuration', { method: 'PUT', body: JSON.stringify(payload) });
    } else if (form.id === 'application-config-form') {
      const storage = configuration.objectStorage;
      configuration = await api('/system/configuration', {
        method: 'PUT',
        body: JSON.stringify({
          s3Endpoint: storage.endpoint || '', s3Bucket: storage.bucket || '', s3Region: storage.region || 'garage',
          s3UseTLS: Boolean(storage.tls), s3PathStyle: Boolean(storage.pathStyle), s3CreateBucket: Boolean(storage.createBucket),
          pdfAiUrl: data.pdfAiUrl.trim(), transactionAiUrl: data.transactionAiUrl.trim(), transactionAiModel: data.transactionAiModel || transactionAIModels[0] || defaultTransactionAIModel,
        }),
      });
    } else if (form.id === 'backup-form') {
      const rcloneConfig = data.rcloneConfig.trim();
      const resticPassword = data.resticPassword;
      const payload = { repository: data.repository.trim(), policy: { schedule: data.schedule, keepDaily: Number(data.keepDaily), keepWeekly: Number(data.keepWeekly), keepMonthly: Number(data.keepMonthly) } };
      if (rcloneConfig) payload.rcloneConfig = rcloneConfig;
      if (resticPassword) payload.resticPassword = resticPassword;
      backupStatus = await api('/system/backup', { method: 'PUT', body: JSON.stringify(payload) });
      backupMessage = 'Backupconfiguratie is opgeslagen en gecontroleerd.';
    } else if (form.id === 'recovery-form') {
      backupStatus = await api('/system/backup/recovery', { method: 'POST', body: JSON.stringify({ snapshotId: data.snapshotId.trim(), confirmation: data.confirmation.trim() }) });
      backupMessage = 'Recovery is gepland. De applicatie sluit af zodat herstel bij de volgende start kan worden uitgevoerd.';
    } else if (form.id === 'bank-config-form') {
      selectedBankConfig = await api(`/bank-configs/${encodeURIComponent(data.slug.trim())}`, { method: 'PUT', body: JSON.stringify({ slug: data.slug.trim(), rawYaml: data.rawYaml }) });
      bankConfigs = await api('/bank-configs');
      bankConfigMessage = 'Bankconfiguratie is opgeslagen en gesynchroniseerd met bank_import_profiles.';
    } else if (form.id === 'bank-import-form') {
      bankImportSelectedConfig = data.bankConfig;
      bankImportFile = form.elements.file.files[0] || bankImportFile;
      if (!bankImportFile) throw new Error('Kies een bankbestand.');
      const upload = new FormData();
      upload.set('bankConfig', bankImportSelectedConfig);
      upload.set('file', bankImportFile);
      const response = await fetch(`${apiBaseUrl}/api/v1/bank-imports/preview`, { method: 'POST', body: upload });
      if (!response.ok) throw new Error((await response.json().catch(() => ({}))).error || `Preview mislukt (${response.status})`);
      bankImportPreview = await response.json();
      bankImportMessage = 'Preview is gemaakt. Het geselecteerde bestand blijft bewaard; je kunt nu importeren zonder opnieuw te kiezen.';
    } else if (form.id === 'transaction-filter-form') {
      transactionViewFilter = data.view;
      transactionFromFilter = data.from;
      transactionToFilter = data.to;
      transactionSearchFilter = data.search;
      transactionOffset = 0;
    }
  });
});

app.textContent = 'Administratie laden…';
window.addEventListener('hashchange', activateTab);
refresh().catch(showError);
