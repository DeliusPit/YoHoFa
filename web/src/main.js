import './core.css';

const app = document.querySelector('#app');
const tabLabels = {
  dashboard: 'Dashboard',
  budget: 'Budget',
  'balans-planning': 'Balans & Planning',
  'inkomsten-uitgaven': 'Inkomsten & Uitgaven',
  documenten: 'Documenten',
  import: 'Import',
  configuratie: 'Configuratie',
};
let activeTab = Object.hasOwn(tabLabels, window.location.hash.slice(1)) ? window.location.hash.slice(1) : 'balans-planning';
let uiLanguage = 'nl';
let uiTranslations = {};
const t = (key, fallback) => uiTranslations[key] || fallback;
const tabTranslationKeys = { dashboard: 'nav.dashboard', budget: 'nav.budget', 'balans-planning': 'nav.balance', 'inkomsten-uitgaven': 'nav.transactions', documenten: 'nav.documents', import: 'nav.import', configuratie: 'nav.configuration' };
const tabLabel = (id) => t(tabTranslationKeys[id], tabLabels[id]);
const languageNames = { nl: 'Nederlands', en: 'English', de: 'Deutsch', fr: 'Français' };
const typeLabel = (type) => t(`type.${type}`, types[type] || type);
const createObjectLabel = (type) => t(`object.new.${type}`, objectSections[type]?.create || 'Nieuw object');
const tt = (value) => uiTranslations[`text.${value}`] || value;

function iconButton(label, icon, attributes = '', classes = 'secondary icon-action') {
  const safeLabel = escapeHtml(label);
  return `<button type="button" class="${classes}" ${attributes} aria-label="${safeLabel}" title="${safeLabel}"><span aria-hidden="true">${escapeHtml(icon)}</span></button>`;
}
document.title = `${tabLabel(activeTab)} · YoHoFa`;
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || '';
const types = { asset: 'Bezit', debt: 'Schuld', expense: 'Last', reserve: 'Reservering', financial: 'Financieel' };
const objectSections = {
  asset: { title: 'Bezittingen', create: 'Nieuw bezit', id: 'balance-assets' },
  debt: { title: 'Schulden', create: 'Nieuwe schuld', id: 'balance-debts' },
  expense: { title: 'Lasten', create: 'Nieuwe last', id: 'balance-expenses' },
  reserve: { title: 'Reserveringen', create: 'Nieuwe reservering', id: 'balance-reserves' },
  financial: { title: 'Financieel', create: 'Nieuw financieel object', id: 'balance-financial' },
};
const configurationSections = {
  application: 'Applicatie Configuratie',
  database: 'Database configuratie',
  ai: 'AI-Configuratie',
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
let objectTransactionsTotal = 0;
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
let selectedTransactionIDs = new Set();
const transactionPageSize = 25;
const dashboardMonths = ['Jan', 'Feb', 'Mrt', 'Apr', 'Mei', 'Jun', 'Jul', 'Aug', 'Sep', 'Okt', 'Nov', 'Dec'];
let dashboard = null;
let dashboardYear = new Date().getFullYear();
let dashboardMonth = 0;
let dashboardLoading = true;
let dashboardError = '';
let dashboardRequestID = 0;
let budget = null;
let budgetYear = new Date().getFullYear();
let budgetMonth = 0;
let budgetLoading = true;
let budgetError = '';
let budgetRequestID = 0;
let configuration = null;
let configurationLoading = true;
let configurationError = '';
let configurationRequestID = 0;
let configurationFocus = 'application';
let transactionAIModels = [];
let transactionAIModelsLoading = false;
let transactionAIModelsError = '';
let documentAIModels = [];
let documentAIModelsLoading = false;
let documentAIModelsError = '';
let backupStatus = null;
let backupSnapshots = [];
let backupMessage = '';
let bankConfigs = [];
let selectedBankConfig = null;
let bankConfigMessage = '';
let aiDocsConfig = null;
let aiDocsConfigMessage = '';
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
let selectedDocumentID = null;
let documentSearchFilter = '';
const selectedDocumentIDs = new Set();

function escapeHtml(value) {
  return String(value ?? '').replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]);
}

function translateStaticDOM(root = app) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const nodes = [];
  while (walker.nextNode()) nodes.push(walker.currentNode);
  for (const node of nodes) {
    if (node.parentElement?.closest('tbody, dd, option, code, pre, .object-list, .document-list, .bank-config-list')) continue;
    const translated = tt(node.nodeValue.trim());
    if (translated !== node.nodeValue.trim()) node.nodeValue = node.nodeValue.replace(node.nodeValue.trim(), translated);
  }
  for (const element of root.querySelectorAll('[placeholder], [aria-label], [title]')) {
    for (const attribute of ['placeholder', 'aria-label', 'title']) {
      const value = element.getAttribute(attribute);
      if (value) element.setAttribute(attribute, tt(value));
    }
  }
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
  // A bulk selection belongs to the currently visible transaction page only.
  selectedTransactionIDs.clear();
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
      api(`/transactions?financialObjectId=${selection}&limit=100&sort=date&order=desc`),
    ]);
  }
  if (currentRequest !== requestID) return;
  objects = nextObjects;
  summary = nextSummary;
  selectedID = nextObjects.some((object) => object.id === selection) ? selection : null;
  values = nextValues;
  relations = nextRelations;
  objectTransactions = nextObjectTransactions.items || [];
  objectTransactionsTotal = nextObjectTransactions.total || objectTransactions.length;
  transactions = nextTransactions.items;
  transactionTotals = nextTransactions;
  documents = nextDocuments;
  render();
  await Promise.all([loadDashboard(), loadBudget(), loadConfiguration()]);
}

function render() {
  const selected = objects.find((object) => object.id === selectedID);
  app.innerHTML = `<div class="core-shell">
    <nav class="core-navigation" aria-label="Hoofdnavigatie">${Object.keys(tabLabels).map((id) => `<a href="#${id}" data-tab="${id}" ${activeTab === id ? 'aria-current="page"' : ''}>${escapeHtml(tabLabel(id))}</a>`).join('')}</nav>
    <p id="message" class="error" role="alert">${escapeHtml(message)}</p>
    <section id="dashboard" class="core-tab-section" aria-labelledby="dashboard-heading" ${activeTab === 'dashboard' ? '' : 'hidden'}>
      <header><p class="eyebrow">Your Own Household Financial Assistant · v3</p><h1 id="dashboard-heading">${escapeHtml(tabLabel('dashboard'))}</h1>
        <p>${escapeHtml(t('page.dashboard.subtitle', 'Je balans, planning en geldstromen voor de gekozen periode.'))}</p></header>
      <div id="dashboard-content">${renderDashboard()}</div>
    </section>
    <section id="budget" class="core-tab-section" aria-labelledby="budget-heading" ${activeTab === 'budget' ? '' : 'hidden'}>
      <header><p class="eyebrow">Your Own Household Financial Assistant · v3</p><h1 id="budget-heading">${escapeHtml(tabLabel('budget'))}</h1></header>
      <div id="budget-content">${renderBudget()}</div>
    </section>
    <section id="balans-planning" class="core-tab-section" aria-labelledby="balans-heading" ${activeTab === 'balans-planning' ? '' : 'hidden'}>
    <header><p class="eyebrow">Your Own Household Financial Assistant · v3</p><h1 id="balans-heading">${escapeHtml(tabLabel('balans-planning'))}</h1>
      <p>${escapeHtml(t('page.balance.subtitle', 'Beheer je bezittingen, schulden, lasten, reserveringen en financiële mutaties.'))}</p></header>
    <p class="balance-overview">${summary.financialObjects ?? 0} financiële objecten · ${summary.objectValues ?? 0} waarderingen · ${summary.objectRelations ?? 0} relaties</p>
    <div class="balance-toolbar">
      <div class="balance-subnavigation" role="group" aria-label="Objectsoorten"><button type="button" class="period-button ${objectTypeFocus === 'all' ? 'active' : ''}" data-action="object-type" data-type="all" aria-pressed="${objectTypeFocus === 'all'}">${escapeHtml(t('object.all', 'Alle objecten'))}</button>${Object.keys(types).map((type) => `<button type="button" class="period-button ${objectTypeFocus === type ? 'active' : ''}" data-action="object-type" data-type="${type}" aria-pressed="${objectTypeFocus === type}">${escapeHtml(typeLabel(type))}</button>`).join('')}</div>
      <div class="balance-create-actions" role="group" aria-label="Nieuw financieel object">${Object.keys(objectSections).map((type) => `<button type="button" class="balance-create-button ${type}" data-action="new" data-type="${type}">${escapeHtml(createObjectLabel(type))}</button>`).join('')}</div>
    </div>
    <div class="balance-object-grid ${objectTypeFocus === 'all' ? '' : 'single'}">${Object.entries(objectSections).filter(([type]) => objectTypeFocus === 'all' || objectTypeFocus === type).map(([type, section]) => renderObjectFrame(type, section)).join('')}</div>
    ${objectEditorOpen ? renderObjectEditor(selected) : ''}
    </section>
    <section id="inkomsten-uitgaven" class="core-tab-section" aria-labelledby="inkomsten-heading" ${activeTab === 'inkomsten-uitgaven' ? '' : 'hidden'}>
      <header><p class="eyebrow">Your Own Household Financial Assistant · v3</p><h1 id="inkomsten-heading">${escapeHtml(tabLabel('inkomsten-uitgaven'))}</h1>
        <p>${escapeHtml(t('page.transactions.subtitle', 'Beheer je ontvangsten en betalingen en verbind ze met financiële objecten.'))}</p></header>
      ${renderTransactions()}
    </section>
    <section id="documenten" class="core-tab-section" aria-labelledby="documenten-heading" ${activeTab === 'documenten' ? '' : 'hidden'}>
      <header><p class="eyebrow">Your Own Household Financial Assistant · v3</p><h1 id="documenten-heading">${escapeHtml(tabLabel('documenten'))}</h1>
      </header>
      <div id="document-content">${renderDocuments()}</div>
    </section>
    <section id="import" class="core-tab-section" aria-labelledby="import-heading" ${activeTab === 'import' ? '' : 'hidden'}>
      <header><p class="eyebrow">Your Own Household Financial Assistant · v3</p><h1 id="import-heading">${escapeHtml(tabLabel('import'))}</h1>
        <p>${escapeHtml(t('page.import.subtitle', 'Bankgegevens omzetten naar genormaliseerde transacties.'))}</p></header>
      <div id="bank-import-content">${renderBankImport()}</div>
    </section>
    <section id="configuratie" class="core-tab-section" aria-labelledby="configuratie-heading" ${activeTab === 'configuratie' ? '' : 'hidden'}>
      <header><p class="eyebrow">Your Own Household Financial Assistant · v3</p><h1 id="configuratie-heading">${escapeHtml(tabLabel('configuratie'))}</h1>
      </header>
      <div id="configuration-content">${renderConfiguration()}</div>
    </section>
  </div>`;
  translateStaticDOM(app);
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
      <div class="core-actions"><button type="submit" ${validConfigs.length && bankImportFile ? '' : 'disabled'}>${escapeHtml(t('action.preview', 'Preview maken'))}</button><button type="button" data-action="import-bank-transactions" ${validConfigs.length && bankImportFile ? '' : 'disabled'}>${escapeHtml(t('action.importTransactions', 'Transacties importeren'))}</button></div>
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
    ${configurationFocus === 'ai' ? renderAIConfiguration(cfg) : ''}
    ${configurationFocus === 'bankimport' ? renderBankimportConfiguration() : ''}
  </div>`;
}

function renderDatabaseConfiguration(cfg, databaseSize, statusBadge) {
  const databaseOptions = (cfg.database.availableDatabases || [cfg.database.filename || 'hohoho.db']).map((name) => `<option value="${escapeHtml(name)}" ${name === cfg.database.filename ? 'selected' : ''}>${escapeHtml(name)}</option>`).join('');
  return `<section class="core-panel configuration-panel" aria-labelledby="database-config-heading">
      <div class="core-heading"><h2 id="database-config-heading">Databases en opslag</h2>${statusBadge(cfg.database.status === 'ok', cfg.database.status === 'ok' ? 'SQLite bereikbaar' : 'SQLite fout')}</div>
      <p class="muted">V3 gebruikt één vaste opslagcombinatie: SQLite voor gestructureerde data en Garage/S3 voor documentbytes. S3-wijzigingen worden opgeslagen en gelden na een herstart.</p>
      ${cfg.restartRequired ? '<p class="config-notice">Configuratie opgeslagen. Herstart de applicatie om de gewijzigde S3-instellingen actief te maken.</p>' : ''}
      <dl class="configuration-list">
        <div><dt>Database backend</dt><dd>SQLite</dd></div>
        <div><dt>Databasepad</dt><dd><code>${escapeHtml(cfg.database.path)}</code></dd></div>
        <div><dt>Datamap</dt><dd><code>${escapeHtml(cfg.database.directory || '/app/data')}</code></dd></div>
        <div><dt>Schemaversie</dt><dd>${cfg.database.schemaVersion}</dd></div>
        <div><dt>Databasegrootte</dt><dd>${databaseSize}</dd></div>
        <div><dt>S3 toegang</dt><dd>${cfg.objectStorage.accessKeySet ? 'Access key aanwezig' : 'Access key ontbreekt'} · ${cfg.objectStorage.secretKeySet ? 'Secret key aanwezig' : 'Secret key ontbreekt'}</dd></div>
        <div><dt>Objectopslagstatus</dt><dd>${statusBadge(cfg.objectStorage.ready, cfg.objectStorage.statusMessage || (cfg.objectStorage.ready ? 'Bereikbaar' : 'Niet bereikbaar'))}</dd></div>
      </dl>
      <form id="configuration-form" class="configuration-form">
        <h3>SQLite database kiezen</h3>
        <label>Database <select name="databaseFile">${databaseOptions}</select></label>
        <p class="muted">De database staat altijd in de datamap. Een andere keuze wordt actief na een herstart.</p>
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
        <div class="core-actions"><button type="submit">${escapeHtml(t('action.saveS3', 'S3-configuratie opslaan'))}</button></div>
      </form>
    </section>`;
}

function renderApplicationConfiguration(cfg) {
  const languages = cfg.runtime.availableLanguages || [];
  const languageOptions = languages.map((language) => `<option value="${escapeHtml(language)}" ${language === cfg.runtime.language ? 'selected' : ''}>${escapeHtml(language === uiLanguage ? (uiTranslations['language.name'] || languageNames[language] || language) : (languageNames[language] || language))}</option>`).join('');
  return `<section class="core-panel configuration-panel" aria-labelledby="runtime-config-heading">
      <h2 id="runtime-config-heading">${escapeHtml(t('config.application.title', 'Applicatie'))}</h2>
      <dl class="configuration-list">
        <div><dt>${escapeHtml(t('config.runtime.environment', 'Omgeving'))}</dt><dd>${escapeHtml(cfg.runtime.environment)}</dd></div>
        <div><dt>${escapeHtml(t('config.runtime.httpAddress', 'HTTP-adres'))}</dt><dd><code>${escapeHtml(cfg.runtime.httpAddress)}</code></dd></div>
        <div><dt>${escapeHtml(t('config.runtime.webDirectory', 'Frontendmap'))}</dt><dd><code>${escapeHtml(cfg.runtime.webDirectory)}</code></dd></div>
        <div><dt>${escapeHtml(t('config.runtime.authentication', 'Authenticatie'))}</dt><dd>${cfg.runtime.authenticationEnabled ? escapeHtml(t('config.status.enabled', 'Ingeschakeld')) : escapeHtml(t('config.status.disabled', 'Uitgeschakeld'))}</dd></div>
        <div><dt>${escapeHtml(t('config.runtime.cors', 'CORS'))}</dt><dd>${cfg.runtime.corsConfigured ? escapeHtml(t('config.status.configured', 'Geconfigureerd')) : escapeHtml(t('config.status.unset', 'Niet ingesteld'))}</dd></div>
        <div><dt>PDF/OCR-AI</dt><dd>${cfg.runtime.pdfAiConfigured ? escapeHtml(t('config.status.configured', 'Geconfigureerd')) : escapeHtml(t('config.status.unset', 'Niet ingesteld'))}</dd></div>
        <div><dt>Transactie-AI</dt><dd>${cfg.runtime.transactionAiConfigured ? escapeHtml(t('config.status.configured', 'Geconfigureerd')) : escapeHtml(t('config.status.unset', 'Niet ingesteld'))}</dd></div>
      </dl>
      ${cfg.restartRequired ? '<p class="config-notice">Configuratie opgeslagen. Herstart de applicatie om gewijzigde serverkoppelingen actief te maken.</p>' : ''}
      <form id="application-config-form" class="configuration-form">
        ${languages.length ? `<label>${escapeHtml(t('config.application.language', 'Taal'))} <select name="language">${languageOptions}</select></label>` : ''}
        <div class="core-actions"><button type="submit">${escapeHtml(t('config.application.save', 'Applicatieconfiguratie opslaan'))}</button><button type="button" class="secondary" data-action="restart-application">Applicatie herstarten</button></div>
      </form>
    </section>`;
}

function renderAIConfiguration(cfg) {
  const documentAIURL = cfg.runtime.pdfAiUrl || '';
  const documentAIModel = cfg.runtime.pdfAiModel || 'Qwen3.8-Flash-Next-UD-Q4_K_XL';
  const documentModels = documentAIModels.length ? documentAIModels : [documentAIModel];
  const documentModelOptions = [...new Set([documentAIModel, ...documentModels])].map((model) => `<option value="${escapeHtml(model)}" ${model === documentAIModel ? 'selected' : ''}>${escapeHtml(model)}</option>`).join('');
  const documentModelStatus = documentAIModelsLoading ? 'Modellen ophalen...' : documentAIModelsError || (documentAIModels.length ? `${documentAIModels.length} modellen gevonden.` : 'Vul de URL in en haal de beschikbare modellen op.');
  const transactionAIURL = cfg.runtime.transactionAiUrl || '';
  const transactionAIModel = cfg.runtime.transactionAiModel || transactionAIModels[0] || defaultTransactionAIModel;
  const availableModels = transactionAIModels.length ? transactionAIModels : [transactionAIModel];
  const modelOptions = [...new Set([transactionAIModel, ...availableModels])].map((model) => `<option value="${escapeHtml(model)}" ${model === transactionAIModel ? 'selected' : ''}>${escapeHtml(model)}</option>`).join('');
  const modelStatus = transactionAIModelsLoading ? 'Modellen ophalen...' : transactionAIModelsError || (transactionAIModels.length ? `${transactionAIModels.length} modellen gevonden.` : 'Vul de URL in en haal de beschikbare modellen op.');
  return `${renderAIConnectionsConfiguration(cfg, documentAIURL, documentAIModel, documentModelOptions, documentModelStatus, transactionAIURL, modelOptions, modelStatus)}${renderAIDocsConfiguration()}`;
}

function renderAIConnectionsConfiguration(cfg, documentAIURL, documentAIModel, documentModelOptions, documentModelStatus, transactionAIURL, modelOptions, modelStatus) {
  const transactionAIModel = cfg.runtime.transactionAiModel || transactionAIModels[0] || defaultTransactionAIModel;
  const reuseDocumentAI = !transactionAIURL || (documentAIURL && transactionAIURL === documentAIURL && transactionAIModel === documentAIModel);
  return `<section class="core-panel configuration-panel" aria-labelledby="ai-config-heading">
    <h2 id="ai-config-heading">AI-Configuratie</h2>
    ${cfg.restartRequired ? '<p class="config-notice">Configuratie opgeslagen. Herstart de applicatie om gewijzigde serverkoppelingen actief te maken.</p>' : ''}
    <form id="ai-runtime-config-form" class="configuration-form">
      <h3>${escapeHtml(t('config.application.aiLinks', 'AI-koppelingen'))}</h3>
      <label>${escapeHtml(t('config.application.documentAiUrl', 'Document-AI URL'))} <input name="pdfAiUrl" value="${escapeHtml(documentAIURL)}" placeholder="http://ai.example.local:8081/v1/chat/completions" /></label>
      <div id="document-ai-model-row" class="configuration-inline" ${documentAIURL ? '' : 'hidden'}>
        <label>${escapeHtml(t('config.application.documentAiModel', 'Document-AI model'))} <select name="pdfAiModel">${documentModelOptions}</select></label>
        <button type="button" data-action="load-document-ai-models">${escapeHtml(t('config.application.loadModels', 'Modellen ophalen'))}</button>
      </div>
      <p id="document-ai-model-status" class="muted" ${documentAIURL ? '' : 'hidden'}>${escapeHtml(documentModelStatus)}</p>
      <label class="checkbox-label"><input name="reuseDocumentAI" type="checkbox" ${reuseDocumentAI ? 'checked' : ''} /> ${escapeHtml(t('config.application.reuseDocumentAI', 'Document-URL/model ook gebruiken voor transacties'))}</label>
      <div id="transaction-ai-settings" ${reuseDocumentAI ? 'hidden' : ''}>
      <label>${escapeHtml(t('config.application.transactionAiUrl', 'Transactieclassificatie-AI URL'))} <input name="transactionAiUrl" value="${escapeHtml(reuseDocumentAI ? documentAIURL : transactionAIURL)}" placeholder="http://ai.example.local:8080/v1/chat/completions" /></label>
      <div id="transaction-ai-model-row" class="configuration-inline" ${(reuseDocumentAI ? documentAIURL : transactionAIURL) ? '' : 'hidden'}>
        <label>${escapeHtml(t('config.application.transactionAiModel', 'Transactie-AI model'))} <select name="transactionAiModel">${modelOptions}</select></label>
        <button type="button" data-action="load-transaction-ai-models">${escapeHtml(t('config.application.loadModels', 'Modellen ophalen'))}</button>
      </div>
      <p id="transaction-ai-model-status" class="muted" ${transactionAIURL ? '' : 'hidden'}>${escapeHtml(modelStatus)}</p>
      </div>
      <p class="muted">De PDF/OCR-AI wordt gebruikt voor documentverwerking. De transactie-AI is bedoeld voor automatische classificatie van banktransacties.</p>
      <div class="core-actions"><button type="submit">${escapeHtml(t('config.application.saveAI', 'AI-configuratie opslaan'))}</button></div>
    </form>
  </section>`;
}

function renderBankimportConfiguration() {
  const selected = selectedBankConfig || bankConfigs[0] || null;
  return `<section class="core-panel configuration-panel" aria-labelledby="bankimport-config-heading">
    <div class="core-heading"><h2 id="bankimport-config-heading">Bankimport Configuratie</h2><button type="button" data-action="new-bank-config">${escapeHtml(t('action.newBankConfig', 'Nieuwe bankconfig'))}</button></div>
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
        <div class="core-actions"><button type="submit">${escapeHtml(t('action.saveBankConfig', 'Bankconfig opslaan'))}</button>${selected?.slug ? `<button type="button" class="secondary" data-action="delete-bank-config" data-slug="${escapeHtml(selected.slug)}">${escapeHtml(t('action.delete', 'Verwijderen'))}</button>` : ''}</div>
      </form>
    </div>
  </section>`;
}

function renderAIDocsConfiguration() {
  const cfg = aiDocsConfig || { rawYaml: '', status: { valid: false, errors: ['Configuratie wordt geladen.'] }, path: '' };
  return `<section class="core-panel configuration-panel" aria-labelledby="aidocs-config-heading">
    <div class="core-heading"><h2 id="aidocs-config-heading">Document-AI Configuratie</h2><span class="config-status ${cfg.status?.valid ? 'ok' : 'warning'}">${cfg.status?.valid ? 'geldig' : 'ongeldig'}</span></div>
    <p class="muted">Deze YAML bepaalt hoe YoHoFa documenten verwerkt: tekstextractie/OCR-instellingen, het taalmodel en prompts voor vertaling en analyse.</p>
    ${cfg.path ? `<p class="muted">Bestand: <code>${escapeHtml(cfg.path)}</code></p>` : ''}
    ${aiDocsConfigMessage ? `<p class="config-notice">${escapeHtml(aiDocsConfigMessage)}</p>` : ''}
    <form id="ai-docs-config-form" class="configuration-form">
      <label>YAML-configuratie <textarea name="rawYaml" rows="26" spellcheck="false" required>${escapeHtml(cfg.rawYaml || '')}</textarea></label>
      ${cfg.status?.errors?.length ? `<p class="error">${cfg.status.errors.map(escapeHtml).join('\n')}</p>` : ''}
      <div class="core-actions"><button type="submit">${escapeHtml(t('action.saveAIDocsConfig', 'Document-AI config opslaan'))}</button></div>
    </form>
  </section>`;
}

function renderBackupConfiguration(cfg, statusBadge) {
  const status = backupStatus || {};
  const policy = status.policy || { schedule: '02:30', keepDaily: 14, keepWeekly: 8, keepMonthly: 24 };
  const ready = cfg.backupRecovery.available;
  const [configuredRemote, configuredPath] = String(status.repository || '').split(':', 2);
  const [configuredBucket, ...configuredFolder] = (configuredPath || '').split('/').filter(Boolean);
  return `<section class="core-panel configuration-panel" aria-labelledby="backup-config-heading">
    <div class="core-heading"><h2 id="backup-config-heading">Backup en recovery</h2>${statusBadge(ready && status.configured, ready ? (status.configured ? 'Geconfigureerd' : 'Beschikbaar') : 'Niet beschikbaar')}</div>
    <p class="muted">Bescherm je administratie met een versleutelde kopie van SQLite, documenten en instellingen. Elke geslaagde backup wordt gecontroleerd met een restore-proef.</p>
    ${ready ? '' : `<p class="config-notice">${escapeHtml(cfg.backupRecovery.message)}</p>`}
    ${backupMessage ? `<p class="config-notice">${escapeHtml(backupMessage)}</p>` : ''}
    <form id="backup-form" class="configuration-form">
      <fieldset class="backup-step">
        <legend><span>1</span> Stel een opslaglocatie in</legend>
        <p>Maak een rclone-remote voor een S3-compatibele opslagdienst. Gebruik een andere bucket dan die van je documenten.</p>
        <div class="backup-storage-grid">
          <label>Naam van de verbinding <input name="remoteName" value="${escapeHtml(configuredRemote || 'backup')}" pattern="[A-Za-z0-9_-]+" placeholder="backup" ${ready ? '' : 'disabled'} required /><small>Een korte naam, bijvoorbeeld <code>backup</code>.</small></label>
          <label>S3-endpoint <input name="s3Endpoint" type="url" placeholder="https://s3.example.nl" ${ready ? '' : 'disabled'} /><small>Het endpoint van je opslagdienst.</small></label>
          <label>Access key <input name="s3AccessKey" autocomplete="new-password" placeholder="${status.rcloneConfigSet ? 'Alleen invullen om te vervangen' : 'Vul de access key in'}" ${ready ? '' : 'disabled'} /></label>
          <label>Secret key <input name="s3SecretKey" type="password" autocomplete="new-password" placeholder="${status.rcloneConfigSet ? 'Alleen invullen om te vervangen' : 'Vul de secret key in'}" ${ready ? '' : 'disabled'} /></label>
          <label>Bucket <input name="bucket" value="${escapeHtml(configuredBucket || '')}" placeholder="hohoho-backups" ${ready ? '' : 'disabled'} required /></label>
          <label>Map in de bucket <input name="backupFolder" value="${escapeHtml(configuredFolder.join('/') || 'administratie')}" placeholder="administratie" ${ready ? '' : 'disabled'} required /></label>
        </div>
        <label>Backupwachtwoord <input name="resticPassword" type="password" autocomplete="new-password" placeholder="${status.resticPasswordSet ? 'Ongewijzigd laten' : 'Kies een nieuw wachtwoord'}" ${ready ? '' : 'disabled'} /><small>Bewaar dit wachtwoord buiten YoHoFa. Zonder dit wachtwoord is herstel onmogelijk.</small></label>
        <details class="backup-advanced">
          <summary>Geavanceerd: eigen rclone-configuratie gebruiken</summary>
          <p>Alleen nodig voor een andere opslagsoort of een afwijkende rclone-instelling. Deze configuratie vervangt de gegevens hierboven.</p>
          <label>Rclone-configuratie <textarea name="rcloneConfig" rows="8" placeholder="[backup]\ntype = s3\nprovider = Other\nendpoint = https://s3.example.nl\n..." ${ready ? '' : 'disabled'}></textarea></label>
        </details>
      </fieldset>
      <fieldset class="backup-step">
        <legend><span>2</span> Bepaal het bewaarbeleid</legend>
        <p>YoHoFa maakt dagelijks een backup en ruimt oudere snapshots automatisch op volgens deze aantallen.</p>
        <div class="backup-policy-grid">
          <label>Dagelijks tijdstip <input name="schedule" type="time" value="${escapeHtml(policy.schedule)}" ${ready ? '' : 'disabled'} required /></label>
          <label>Dagelijkse backups <input name="keepDaily" type="number" min="1" max="365" value="${escapeHtml(policy.keepDaily)}" ${ready ? '' : 'disabled'} required /></label>
          <label>Wekelijkse backups <input name="keepWeekly" type="number" min="1" max="520" value="${escapeHtml(policy.keepWeekly)}" ${ready ? '' : 'disabled'} required /></label>
          <label>Maandelijkse backups <input name="keepMonthly" type="number" min="1" max="120" value="${escapeHtml(policy.keepMonthly)}" ${ready ? '' : 'disabled'} required /></label>
        </div>
      </fieldset>
      <div class="backup-actions"><button type="submit" ${ready ? '' : 'disabled'}>${escapeHtml(t('action.saveAndValidateBackup', 'Opslaan en verbinding controleren'))}</button><button type="button" class="secondary" data-action="backup-validate" ${ready && status.configured ? '' : 'disabled'}>${escapeHtml(t('action.revalidateBackup', 'Verbinding opnieuw controleren'))}</button><button type="button" data-action="backup-run" ${ready && status.validated && !status.running ? '' : 'disabled'}>${escapeHtml(t('action.runBackup', 'Nu een backup maken'))}</button></div>
    </form>
    <dl class="configuration-list backup-status-list">
      <div><dt>Status</dt><dd>${status.running ? 'Backup draait' : (status.validated ? 'Configuratie gevalideerd' : 'Nog niet gevalideerd')}</dd></div>
       <div><dt>Laatste snapshot</dt><dd>${escapeHtml(status.lastSnapshotId || 'Geen')}</dd></div>
       <div><dt>Laatste controle</dt><dd>${escapeHtml(status.lastValidatedAt || 'Geen')}</dd></div>
       <div><dt>Laatste restore-proef</dt><dd>${escapeHtml(status.lastDrillAt || 'Geen')}</dd></div>
      <div><dt>Laatste fout</dt><dd>${escapeHtml(status.lastError || status.recoveryError || 'Geen')}</dd></div>
      <div><dt>Recovery</dt><dd>${escapeHtml(status.recoveryState || 'Geen herstel gepland')}</dd></div>
    </dl>
    <section class="core-detail backup-recovery"><div class="core-heading"><div><h3>Herstellen</h3><p class="muted">Kies een gecontroleerde snapshot. De applicatie stopt en herstelt de gekozen administratie bij de volgende start.</p></div><button type="button" class="secondary" data-action="backup-snapshots" ${ready && status.configured ? '' : 'disabled'}>${escapeHtml(t('action.refreshSnapshots', 'Snapshots verversen'))}</button></div>
      ${backupSnapshots.length ? `<ul class="core-detail-list">${backupSnapshots.map((snapshot) => `<li><span><strong>${escapeHtml(snapshot.shortId)}</strong> · ${escapeHtml(snapshot.time)}</span><button type="button" class="secondary" data-action="select-recovery-snapshot" data-id="${escapeHtml(snapshot.id)}">${escapeHtml(t('action.selectRecovery', 'Kies voor herstel'))}</button></li>`).join('')}</ul>` : '<p class="muted">Nog geen snapshots geladen.</p>'}
      <form id="recovery-form" class="configuration-form">
        <label>Snapshot ID <input name="snapshotId" placeholder="Snapshot ID" ${ready ? '' : 'disabled'} required /></label>
        <label>Bevestiging <input name="confirmation" placeholder="Typ HERSTEL" ${ready ? '' : 'disabled'} required /></label>
        <button type="submit" class="secondary" ${ready ? '' : 'disabled'}>${escapeHtml(t('action.planRecovery', 'Recovery plannen'))}</button>
      </form>
    </section>
  </section>`;
}

function updateConfiguration() {
  const root = document.querySelector('#configuration-content');
  if (root) {
    root.innerHTML = renderConfiguration();
    translateStaticDOM(root);
  }
}

function updateBankImport() {
  const root = document.querySelector('#bank-import-content');
  if (root) {
    root.innerHTML = renderBankImport();
    translateStaticDOM(root);
  }
}

function renderDocuments() {
  const status = configuration?.objectStorage;
  const batchCount = documents.filter((item) => item.status === 'I' || item.status === 'F').length;
  const selectedCount = selectedDocumentIDs.size;
  return `<section class="core-panel core-prepared" aria-label="Documentenoverzicht"><div class="core-heading"><div class="document-toolbar"><label class="document-filter">Filter op naam <input name="documentSearch" value="${escapeHtml(documentSearchFilter)}" placeholder="Bestandsnaam" data-document-filter /></label><div class="core-actions compact"><button type="button" class="secondary" data-action="refresh-documents">${escapeHtml(t('action.refresh', 'Verversen'))}</button><button type="button" class="secondary" data-action="analyze-selected-documents" ${selectedCount ? '' : 'disabled'}>Geselecteerde AI${selectedCount ? ` (${selectedCount})` : ''}</button><button type="button" class="secondary" data-action="batch-analyze-documents" ${batchCount ? '' : 'disabled'}>${escapeHtml(t('action.batchDocumentAI', 'Batch AI'))}${batchCount ? ` (${batchCount})` : ''}</button><button type="button" data-action="new-document" ${status && !status.ready ? 'disabled' : ''}>${escapeHtml(t('action.newDocument', 'Nieuw document'))}</button></div></div></div>
    ${status && !status.ready ? `<p class="config-notice">${escapeHtml(status.statusMessage || 'S3/Garage is niet bereikbaar; uploaden is uitgeschakeld.')}</p>` : ''}
    ${documentMessage ? `<p class="config-notice">${escapeHtml(documentMessage)}</p>` : ''}
    ${renderDocumentList()}
  </section>
  ${renderDocumentViewer()}
  ${documentUploadOpen ? renderDocumentUploadDialog(status) : ''}`;
}

function renderDocumentUploadDialog(status) {
  return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="document-upload-heading"><div class="core-heading"><h2 id="document-upload-heading">${escapeHtml(t('action.newDocument', 'Nieuw document'))}</h2><button type="button" class="secondary" data-action="close-document-upload">${escapeHtml(t('action.close', 'Sluiten'))}</button></div>
    <form id="document-upload-form" class="configuration-form">
      <label>Bestand <input name="file" type="file" required /></label>
      <div class="core-actions"><button type="submit" ${status && !status.ready ? 'disabled' : ''}>${escapeHtml(t('action.upload', 'Uploaden'))}</button></div>
    </form>
  </section></div>`;
}

function renderDocumentList() {
  if (documentsLoading) return '<p class="muted">Documenten laden...</p>';
  if (documentsError) return `<p class="error">${escapeHtml(documentsError)}</p>`;
  if (!documents.length) return '<p class="muted">Nog geen documenten opgeslagen.</p>';
  const needle = documentSearchFilter.trim().toLowerCase();
  const visibleDocuments = needle ? documents.filter((item) => (item.filename || '').toLowerCase().includes(needle)) : documents;
  if (!visibleDocuments.length) return '<p class="muted">Geen documenten gevonden voor dit filter.</p>';
  return `<ul class="document-list">${visibleDocuments.map((item) => {
    return `<li><label class="document-select" aria-label="Document selecteren"><input type="checkbox" data-document-select="${item.id}" ${selectedDocumentIDs.has(item.id) ? 'checked' : ''} /></label><div><strong>${escapeHtml(item.filename)}</strong><small>Toegevoegd: ${escapeHtml(item.createdAt || '')}</small></div><span class="config-status ${item.status === 'F' ? 'warning' : 'ok'}">${escapeHtml(documentStatuses[item.status] || item.status || 'Onbekend')}</span><div class="document-actions"><button type="button" class="secondary" data-action="open-document" data-id="${item.id}">${escapeHtml(t('action.open', 'Openen'))}</button><button type="button" class="secondary" data-action="analyze-document" data-id="${item.id}">AI</button><button type="button" class="secondary" data-action="manual-document" data-id="${item.id}">${escapeHtml(t('action.classifyManual', 'Handmatig classificeren'))}</button><button type="button" data-action="delete-document" data-id="${item.id}">${escapeHtml(t('action.delete', 'Verwijderen'))}</button></div></li>`;
  }).join('')}</ul>`;
}

function renderDocumentViewer() {
  if (selectedDocumentID == null) return '';
  const item = documents.find((document) => document.id === selectedDocumentID);
  if (!item) return '';
  const analysis = item.analysis || null;
  const summary = analysisSummary(analysis);
  const linkedObjects = (item.financialObjectIds || []).map((id) => objects.find((object) => object.id === id)).filter(Boolean);
  return `<section class="core-panel core-prepared document-viewer" aria-labelledby="document-viewer-heading">
    <div class="core-heading"><div><h2 id="document-viewer-heading">${escapeHtml(item.filename)}</h2><span class="config-status ${item.status === 'F' ? 'warning' : 'ok'}">${escapeHtml(documentStatuses[item.status] || item.status || 'Onbekend')}</span></div><button type="button" class="secondary" data-action="close-document-viewer">${escapeHtml(t('action.close', 'Sluiten'))}</button></div>
    <div class="document-insights">
      <section class="document-translation"><h3>Vertaling</h3><p>${escapeHtml(item.translatedText || 'Geen vertaling beschikbaar.')}</p></section>
      <section><h3>Analyse</h3><p>${escapeHtml(summary || 'Geen analyse beschikbaar.')}</p></section>
      <section><h3>Gekoppelde objecten</h3>${linkedObjects.length ? `<ul class="document-object-links">${linkedObjects.map((object) => `<li><button type="button" class="secondary small-action" data-action="select" data-id="${object.id}">${escapeHtml(object.name)}</button><small>${escapeHtml(typeLabel(object.objectType))}</small></li>`).join('')}</ul>` : '<p>Geen objecten gekoppeld.</p>'}</section>
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
  const available = new Set(documents.map((item) => item.id));
  for (const id of [...selectedDocumentIDs]) {
    if (!available.has(id)) selectedDocumentIDs.delete(id);
  }
  updateDocuments();
}

async function loadSelectedObjectDetails(id) {
  [values, relations, objectTransactions] = [[], [], []];
  objectTransactionsTotal = 0;
  render();
  try {
    const [nextValues, nextRelations, nextObjectTransactions] = await Promise.all([
      api(`/financial-objects/${id}/values`),
      api(`/financial-objects/${id}/relations`),
      api(`/transactions?financialObjectId=${id}&limit=100&sort=date&order=desc`),
    ]);
    if (selectedID !== id) return;
    values = nextValues;
    relations = nextRelations;
    objectTransactions = nextObjectTransactions.items || [];
    objectTransactionsTotal = nextObjectTransactions.total || objectTransactions.length;
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
  if (root) {
    root.innerHTML = renderDocuments();
    translateStaticDOM(root);
  }
}

async function loadConfiguration() {
  const currentRequest = ++configurationRequestID;
  configurationLoading = true;
  configurationError = '';
  updateConfiguration();
  try {
    const result = await api('/system/configuration');
    const [backup, configs, aiConfig, translations] = await Promise.all([api('/system/backup').catch(() => null), api('/bank-configs').catch(() => []), api('/system/ai-docs-config').catch(() => null), loadTranslations(result.runtime?.language || uiLanguage)]);
    if (currentRequest !== configurationRequestID) return;
    configuration = result;
    if (translations) {
      uiLanguage = translations.language || uiLanguage;
      uiTranslations = translations.messages || {};
    }
    backupStatus = backup;
    bankConfigs = configs;
    aiDocsConfig = aiConfig;
    if (selectedBankConfig) selectedBankConfig = bankConfigs.find((item) => item.slug === selectedBankConfig.slug) || selectedBankConfig;
    configurationLoading = false;
    render();
    updateBankImport();
  } catch (error) {
    if (currentRequest !== configurationRequestID) return;
    configurationLoading = false;
    configurationError = error.message;
    updateConfiguration();
  }
}

async function loadTranslations(language) {
  return api(`/system/translations?language=${encodeURIComponent(language || 'nl')}`).catch(() => null);
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

async function loadDocumentAIModels(url) {
  const target = (url || configuration?.runtime?.pdfAiUrl || '').trim();
  if (!target) return;
  documentAIModelsLoading = true;
  documentAIModelsError = '';
  updateAIModelControls();
  try {
    const result = await api(`/system/transaction-ai-models?url=${encodeURIComponent(target)}`);
    documentAIModels = result.models || [];
    documentAIModelsError = documentAIModels.length ? '' : 'Geen modellen gevonden op deze AI-server.';
  } catch (error) {
    documentAIModels = [];
    documentAIModelsError = error.message;
  } finally {
    documentAIModelsLoading = false;
    updateAIModelControls();
  }
}

function updateTransactionAIModelControls() {
  updateAIModelControls();
}

function updateAIModelControls() {
  const form = document.querySelector('#ai-runtime-config-form');
  if (!form) return;
  const documentRow = document.querySelector('#document-ai-model-row');
  const documentStatus = document.querySelector('#document-ai-model-status');
  const documentURL = form.elements.pdfAiUrl.value.trim();
  if (documentRow && documentStatus) {
    documentRow.hidden = !documentURL;
    documentStatus.hidden = !documentURL;
    if (documentURL) {
      const documentSelect = form.elements.pdfAiModel;
      const currentDocument = documentSelect.value || configuration?.runtime?.pdfAiModel || documentAIModels[0] || 'Qwen3.8-Flash-Next-UD-Q4_K_XL';
      if (documentAIModels.length) {
        const models = [...new Set([currentDocument, ...documentAIModels])];
        documentSelect.innerHTML = models.map((model) => `<option value="${escapeHtml(model)}" ${model === currentDocument ? 'selected' : ''}>${escapeHtml(model)}</option>`).join('');
      }
      documentStatus.textContent = documentAIModelsLoading ? 'Modellen ophalen...' : documentAIModelsError || (documentAIModels.length ? `${documentAIModels.length} modellen gevonden.` : 'Vul de URL in en haal de beschikbare modellen op.');
    }
  }
  const settings = document.querySelector('#transaction-ai-settings');
  const row = document.querySelector('#transaction-ai-model-row');
  const status = document.querySelector('#transaction-ai-model-status');
  if (!settings || !row || !status) return;
  const reuse = form.elements.reuseDocumentAI.checked;
  settings.hidden = reuse;
  if (reuse) {
    form.elements.transactionAiUrl.value = documentURL;
    const documentModel = form.elements.pdfAiModel.value || configuration?.runtime?.pdfAiModel || 'Qwen3.8-Flash-Next-UD-Q4_K_XL';
    const models = [...new Set([documentModel, ...transactionAIModels])];
    form.elements.transactionAiModel.innerHTML = models.map((model) => `<option value="${escapeHtml(model)}" ${model === documentModel ? 'selected' : ''}>${escapeHtml(model)}</option>`).join('');
  }
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
    <div class="core-heading"><div><h2 id="object-editor-heading">${escapeHtml(selected.name)}</h2><span class="config-status ok">${escapeHtml(typeLabel(type))}</span></div><button type="button" class="secondary small-action" data-action="close-object-editor">${escapeHtml(t('action.close', 'Sluiten'))}</button></div>
    <dl class="compact-detail-grid">
      <div><dt>Actuele waarde</dt><dd>${selected.currentValue ? money.format(selected.currentValue.valueCents / 100) : 'Geen waarde'}</dd></div>
      <div><dt>Waardedatum</dt><dd>${escapeHtml(selected.currentValue?.valueDate || 'Geen')}</dd></div>
      <div><dt>Omschrijving</dt><dd>${escapeHtml(selected.description || 'Geen')}</dd></div>
    </dl>
    <div class="core-actions compact"><button type="button" data-action="open-object-form">${escapeHtml(t('action.edit', 'Wijzigen'))}</button><button type="button" class="secondary" data-action="open-value-form">${escapeHtml(t('action.addValue', 'Waarde toevoegen'))}</button><button type="button" class="secondary" data-action="open-relation-form">${escapeHtml(t('action.addRelation', 'Relatie toevoegen'))}</button><button type="button" class="secondary" data-action="delete-object" data-id="${selected.id}">${escapeHtml(t('action.delete', 'Verwijderen'))}</button></div>
    ${renderDetails(selected)}
  </section>${objectDialogMode === 'object' ? renderObjectFormDialog(selected, type) : ''}${objectDialogMode === 'value' ? renderValueDialog() : ''}${objectDialogMode === 'relation' ? renderRelationDialog(selected) : ''}`;
}

function renderObjectFormDialog(selected, type) {
  return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="object-dialog-heading">
    <div class="core-heading"><h2 id="object-dialog-heading">${selected ? `${escapeHtml(typeLabel(type))} ${escapeHtml(t('action.edit', 'Wijzigen')).toLowerCase()}` : escapeHtml(createObjectLabel(type))}</h2><button type="button" class="secondary" data-action="close-object-dialog">${escapeHtml(t('action.close', 'Sluiten'))}</button></div>
    <form id="object-form">
      <label>Type <select name="objectType">${Object.keys(types).map((value) => `<option value="${value}" ${type === value ? 'selected' : ''}>${escapeHtml(typeLabel(value))}</option>`).join('')}</select></label>
      <label>Naam <input name="name" value="${escapeHtml(selected?.name)}" required /></label>
      <label>Omschrijving <textarea name="description" rows="3">${escapeHtml(selected?.description)}</textarea></label>
      <div class="core-actions"><button type="submit">${selected ? escapeHtml(t('action.save', 'Opslaan')) : escapeHtml(t('action.create', 'Aanmaken'))}</button></div>
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
      <div class="period-buttons month-buttons" role="group" aria-label="Dashboardmaand"><button type="button" class="period-button ${dashboardMonth === 0 ? 'active' : ''}" data-action="dashboard-month" data-month="0" aria-pressed="${dashboardMonth === 0}">${escapeHtml(t('period.wholeYear', 'Hele jaar'))}</button>${dashboardMonths.map((label, index) => `<button type="button" class="period-button ${dashboardMonth === index + 1 ? 'active' : ''}" data-action="dashboard-month" data-month="${index + 1}" aria-pressed="${dashboardMonth === index + 1}">${label}</button>`).join('')}</div>
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
       ${group('reserve', 'Reserveringen')}${group('expense', 'Lasten')}${group('financial', 'Financieel')}
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
  translateStaticDOM(root);
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

function budgetPeriodLabel() {
  if (!budgetMonth) return `${budgetYear} · hele jaar`;
  return `${new Intl.DateTimeFormat('nl-NL', { month: 'long' }).format(new Date(budgetYear, budgetMonth - 1, 1))} ${budgetYear}`;
}

function renderBudget() {
  const currentYear = new Date().getFullYear();
  const years = [...new Set([currentYear - 1, currentYear, currentYear + 1, budgetYear, ...(budget?.availableYears || [])])]
    .filter((year) => year >= 1 && year <= 9999).sort((left, right) => right - left);
  const ready = !budgetLoading && !budgetError && budget;
  return `<section class="dashboard-period-panel" aria-label="Budgetperiode">
    <div class="period-filter"><span class="period-filter-label">Jaar</span>
      <div class="period-buttons" role="group" aria-label="Budgetjaar">${years.map((year) => `<button type="button" class="period-button ${year === budgetYear ? 'active' : ''}" data-action="budget-year" data-year="${year}" aria-pressed="${year === budgetYear}">${year}</button>`).join('')}</div>
    </div>
    <div class="period-filter"><span class="period-filter-label">Maand</span>
      <div class="period-buttons month-buttons" role="group" aria-label="Budgetmaand"><button type="button" class="period-button ${budgetMonth === 0 ? 'active' : ''}" data-action="budget-month" data-month="0" aria-pressed="${budgetMonth === 0}">${escapeHtml(t('period.wholeYear', 'Hele jaar'))}</button>${dashboardMonths.map((label, index) => `<button type="button" class="period-button ${budgetMonth === index + 1 ? 'active' : ''}" data-action="budget-month" data-month="${index + 1}" aria-pressed="${budgetMonth === index + 1}">${label}</button>`).join('')}</div>
    </div>
    <p class="dashboard-period-label" role="status">${escapeHtml(budgetPeriodLabel())}${budgetLoading ? ' · gegevens laden…' : ''}</p>
  </section>
  ${budgetError ? `<p class="error" role="alert">${escapeHtml(budgetError)}</p>` : ''}
  ${ready ? renderBudgetSummary() : ''}
  <section class="budget-panel" aria-busy="${budgetLoading}">
    ${ready ? renderBudgetItems() : `<p class="muted">${budgetError ? 'Budgetgegevens konden niet worden geladen.' : 'Budgetgegevens worden geladen.'}</p>`}
  </section>`;
}

function renderBudgetSummary() {
  const summary = budget.summary;
  return `<section class="core-stats budget-summary" aria-label="Samenvatting budgetperiode">
    <div><strong>${money.format(summary.assetChangeCents / 100)}</strong><span>Wijziging bezittingen</span></div>
    <div><strong>${money.format(summary.debtChangeCents / 100)}</strong><span>Wijziging schulden</span></div>
    <div><strong>${money.format(summary.expenseBudgetCents / 100)}</strong><span>Begrote lasten</span></div>
    <div><strong class="expense">${money.format(summary.expenseActualCents / 100)}</strong><span>Werkelijke lasten</span></div>
  </section>`;
}

function renderBudgetItems() {
  if (!budget.items.length) return '<p class="muted">Geen financiële objecten met transacties in deze periode.</p>';
  const months = budget.month ? 1 : 12;
  return `<div class="budget-list">${budget.items.map((item) => {
    const isExpense = item.objectType === 'expense';
    const actual = isExpense ? -item.periodAmountCents : item.periodAmountCents;
    const maximum = item.monthlyMaximumCents == null ? null : item.monthlyMaximumCents * months;
    const remaining = maximum == null ? null : maximum - actual;
    const summary = isExpense
      ? `<span class="budget-amounts"><small>Maximum per maand</small><strong>${item.monthlyMaximumCents == null ? 'Geen maximum' : money.format(item.monthlyMaximumCents / 100)}</strong>${budget.month ? '' : `<small>Maximum dit jaar</small><strong>${maximum == null ? 'Geen maximum' : money.format(maximum / 100)}</strong>`}<small>Werkelijk</small><strong class="${remaining != null && remaining < 0 ? 'expense' : ''}">${money.format(actual / 100)}</strong></span>`
      : `<span class="budget-amounts"><small>Toegewezen in periode</small><strong>${money.format(actual / 100)}</strong></span>`;
    const result = !isExpense || maximum == null ? '' : `<span class="budget-result ${remaining < 0 ? 'over' : 'within'}">${remaining < 0 ? `${money.format(-remaining / 100)} over maximum` : `${money.format(remaining / 100)} binnen maximum`}</span>`;
    return `<details class="budget-item"><summary><span><strong>${escapeHtml(item.name)}</strong><small>${escapeHtml(typeLabel(item.objectType))} · ${item.transactions.length} posten</small></span>${summary}${result}</summary>
      <ul class="compact-transaction-list">${item.transactions.map((transaction) => renderCompactTransactionItem(transaction, item.financialObjectId)).join('')}</ul>
    </details>`;
  }).join('')}</div>`;
}

function renderCompactTransactionItem(transaction, currentObjectID) {
  const assigned = transaction.allocationCents == null ? tt('Context') : money.format(transaction.allocationCents / 100);
  const label = t('action.reassign', 'Opnieuw toewijzen');
  return `<li class="compact-transaction-item">
    <div class="compact-transaction-main"><strong>${escapeHtml(dashboardDate(transaction.transactionDate))}</strong><span>${escapeHtml(transaction.description || tt('Geen omschrijving'))}</span>${transaction.counterparty ? `<small>${escapeHtml(transaction.counterparty)}</small>` : ''}</div>
    <div class="compact-transaction-amount"><strong class="${transaction.amountCents > 0 ? 'income' : 'expense'}">${money.format(transaction.amountCents / 100)}</strong><small>${escapeHtml(assigned)}</small></div>
    <form class="transaction-reassign-form" data-transaction-id="${transaction.id}">
      <select name="financialObjectId" aria-label="${escapeHtml(label)}" required>${objects.map((object) => `<option value="${object.id}" ${object.id === currentObjectID ? 'selected' : ''}>${escapeHtml(object.name)} (${escapeHtml(typeLabel(object.objectType))})</option>`).join('')}</select>
      <button type="submit" class="secondary icon-action" aria-label="${escapeHtml(label)}" title="${escapeHtml(label)}"><span aria-hidden="true">↻</span></button>
    </form>
  </li>`;
}

function updateBudget() {
  const root = document.querySelector('#budget-content');
  if (!root) return;
  const openItems = [...root.querySelectorAll('.budget-item[open]')].map((item) => item.querySelector('summary strong')?.textContent);
  root.innerHTML = renderBudget();
  translateStaticDOM(root);
  for (const name of openItems) {
    const item = [...root.querySelectorAll('.budget-item')].find((entry) => entry.querySelector('summary strong')?.textContent === name);
    if (item) item.open = true;
  }
}

async function loadBudget() {
  const currentRequest = ++budgetRequestID;
  const params = new URLSearchParams({ year: budgetYear, month: budgetMonth });
  budgetLoading = true;
  budgetError = '';
  updateBudget();
  try {
    const result = await api(`/budget?${params}`);
    if (currentRequest !== budgetRequestID) return;
    budget = result;
    budgetLoading = false;
    updateBudget();
  } catch (error) {
    if (currentRequest !== budgetRequestID) return;
    budgetLoading = false;
    budgetError = error.message;
    updateBudget();
  }
}

function activateTab() {
  const requested = window.location.hash.slice(1);
  activeTab = Object.hasOwn(tabLabels, requested) ? requested : 'balans-planning';
  document.title = `${tabLabel(activeTab)} · YoHoFa`;
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
    ${values.length ? `<ul class="core-detail-list">${values.map((value) => `<li><span>${escapeHtml(value.valueDate)} · ${money.format(value.valueCents / 100)}</span>${iconButton(t('action.delete', 'Verwijderen'), '×', `data-action="delete-value" data-id="${value.id}"`)}</li>`).join('')}</ul>` : '<p class="muted">Nog geen waarderingen.</p>'}
  </section>
  <section class="core-detail"><h3>Relaties</h3>
    ${relations.length ? `<ul class="core-detail-list">${relations.map((relation, index) => `<li><span>${escapeHtml(objectName(relation.sourceObjectId))} → <strong>${escapeHtml(relation.relationType)}</strong> → ${escapeHtml(objectName(relation.targetObjectId))}</span>${iconButton(t('action.unlink', 'Ontkoppelen'), '×', `data-action="delete-relation" data-index="${index}"`)}</li>`).join('')}</ul>` : '<p class="muted">Nog geen inkomende of uitgaande relaties.</p>'}
  </section>
  <section class="core-detail"><div class="core-heading"><h3>Bijbehorende transacties</h3><a class="dashboard-open-link" href="#inkomsten-uitgaven">Open Inkomsten &amp; Uitgaven</a></div>
    ${objectTransactions.length ? `<p class="muted compact-list-meta">${objectTransactions.length} ${escapeHtml(tt('van'))} ${objectTransactionsTotal} ${escapeHtml(tt('transacties'))}</p><div class="compact-transaction-scroll"><ul class="compact-transaction-list object-transactions">${objectTransactions.map((item) => {
      const link = item.allocations.find((allocation) => allocation.financialObjectId === selected.id);
      return renderCompactTransactionItem({ ...item, allocationCents: link?.amountCents }, selected.id);
    }).join('')}</ul></div>${objectTransactionsTotal > objectTransactions.length ? `<p class="muted compact-list-meta">${objectTransactionsTotal - objectTransactions.length} ${escapeHtml(tt('oudere transacties niet geladen.'))}</p>` : ''}` : '<p class="muted">Nog geen transacties gekoppeld aan dit object.</p>'}
  </section>
  <section class="core-detail"><h3>Documenten</h3>
    ${availableDocuments.length ? `<form id="object-document-link-form" class="core-inline-form"><label>Document <select name="documentId">${availableDocuments.map((document) => `<option value="${document.id}">${escapeHtml(document.filename)} · ${escapeHtml(documentStatuses[document.status] || document.status || 'Onbekend')}</option>`).join('')}</select></label><button type="submit">${escapeHtml(t('action.linkDocument', 'Document koppelen'))}</button></form>` : '<p class="muted">Geen ongekoppelde documenten beschikbaar.</p>'}
    ${linkedDocuments.length ? `<ul class="core-detail-list">${linkedDocuments.map((document) => `<li><span>${escapeHtml(document.filename)} · ${escapeHtml(documentStatuses[document.status] || document.status || 'Onbekend')}</span><div class="core-actions compact">${iconButton(t('action.open', 'Openen'), '↗', `data-action="open-document" data-id="${document.id}"`)}${iconButton(t('action.unlink', 'Ontkoppelen'), '×', `data-action="unlink-object-document" data-id="${document.id}"`)}</div></li>`).join('')}</ul>` : '<p class="muted">Nog geen documenten gekoppeld.</p>'}
  </section>`;
}

function renderValueDialog() {
  const today = new Date().toLocaleDateString('sv-SE');
  return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="value-dialog-heading">
    <div class="core-heading"><h2 id="value-dialog-heading">${escapeHtml(t('action.addValue', 'Waarde toevoegen'))}</h2><button type="button" class="secondary" data-action="close-object-dialog">${escapeHtml(t('action.close', 'Sluiten'))}</button></div>
    <form id="value-form" class="core-inline-form">
      <label>Waardedatum <input name="valueDate" type="date" value="${today}" required /></label>
      <label>Waarde (€) <input name="amount" type="text" inputmode="decimal" placeholder="598500,00" required /></label>
      <button type="submit">${escapeHtml(t('action.add', 'Toevoegen'))}</button>
    </form>
  </section></div>`;
}

function renderRelationDialog(selected) {
  const targets = objects.filter((object) => object.id !== selected.id);
  return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="relation-dialog-heading">
    <div class="core-heading"><h2 id="relation-dialog-heading">${escapeHtml(t('action.addRelation', 'Relatie toevoegen'))}</h2><button type="button" class="secondary" data-action="close-object-dialog">${escapeHtml(t('action.close', 'Sluiten'))}</button></div>
    ${targets.length ? `<form id="relation-form">
      <label>Relatietype <input name="relationType" placeholder="reserved_for" required /></label>
      <label>Naar <select name="targetObjectId">${targets.map((object) => `<option value="${object.id}">${escapeHtml(object.name)} (${escapeHtml(typeLabel(object.objectType))})</option>`).join('')}</select></label>
      <button type="submit">${escapeHtml(t('action.add', 'Toevoegen'))}</button>
    </form>` : '<p class="muted">Maak eerst een tweede object aan.</p>'}
  </section></div>`;
}

function amountInput(value) {
  return value == null ? '' : (Math.abs(value) / 100).toFixed(2).replace('.', ',');
}

function isoDate(year, monthIndex, day) {
  return new Date(Date.UTC(year, monthIndex, day)).toISOString().slice(0, 10);
}

function monthlyTransactionDates(startDate, endDate) {
  const start = new Date(`${startDate}T00:00:00Z`);
  const end = new Date(`${endDate}T00:00:00Z`);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end < start) throw new Error(tt('Kies een einddatum op of na de eerste transactiedatum.'));
  const dates = [];
  const originalDay = start.getUTCDate();
  for (let monthOffset = 0; monthOffset < 121; monthOffset += 1) {
    const year = start.getUTCFullYear();
    const month = start.getUTCMonth() + monthOffset;
    const lastDay = new Date(Date.UTC(year, month + 1, 0)).getUTCDate();
    const candidate = isoDate(year, month, Math.min(originalDay, lastDay));
    if (candidate > endDate) break;
    dates.push(candidate);
  }
  if (dates.length > 120) throw new Error(tt('Maak maximaal 120 maandelijkse transacties tegelijk aan.'));
  return dates;
}

function renderAllocationRow(link = {}) {
  return `<div class="allocation-row">
    <label>Financieel object <select class="allocation-object"><option value="">Kies een object</option>${objects.map((object) => `<option value="${object.id}" ${object.id === link.financialObjectId ? 'selected' : ''}>${escapeHtml(object.name)} (${escapeHtml(typeLabel(object.objectType))})</option>`).join('')}</select></label>
    <label>Toegewezen bedrag (€) <input class="allocation-amount" type="text" inputmode="decimal" value="${amountInput(link.amountCents)}" placeholder="Leeg = alleen koppelen" /></label>
    <div class="core-actions"><button type="button" class="secondary" data-action="allocate-whole">${escapeHtml(t('action.allocateWhole', 'Hele bedrag'))}</button><button type="button" class="secondary" data-action="remove-allocation">${escapeHtml(t('action.removeAllocation', 'Koppeling weghalen'))}</button></div>
  </div>`;
}

function renderTransactions() {
  const draft = transactionDraft;
  const direction = draft ? (draft.amountCents > 0 ? 'income' : 'expense') : (transactionViewFilter === 'income' ? 'income' : 'expense');
  const links = draft?.allocations || (selectedID == null ? [] : [{ financialObjectId: selectedID }]);
  const rows = links.length ? links : (objects.length ? [{}] : []);
  return `<section class="core-panel core-transactions" aria-labelledby="transactions-heading">
    <div class="core-heading"><h2 id="transactions-heading">Transacties</h2><div class="core-actions"><button type="button" data-action="classify-transactions-ai">${escapeHtml(t('action.aiClassify', 'Classificeren met AI'))}</button><button type="button" data-action="new-transaction">${escapeHtml(t('action.newTransaction', 'Nieuwe transactie'))}</button></div></div>
    <p class="muted">Leg een ontvangst of betaling vast en koppel deze aan je financiële objecten. Een koppeling zonder bedrag is alleen context.</p>
    <form id="transaction-filter-form" class="transaction-filter-bar">
      <label>Toon <select name="view"><option value="">${escapeHtml(t('transaction.filter.visible', 'Zichtbare transacties'))}</option><option value="income" ${transactionViewFilter === 'income' ? 'selected' : ''}>${escapeHtml(t('transaction.filter.income', 'Inkomsten'))}</option><option value="expense" ${transactionViewFilter === 'expense' ? 'selected' : ''}>${escapeHtml(t('transaction.filter.expense', 'Uitgaven'))}</option><option value="hidden" ${transactionViewFilter === 'hidden' ? 'selected' : ''}>${escapeHtml(t('transaction.filter.hidden', 'Verborgen transacties'))}</option></select></label>
      <label>Vanaf <input name="from" type="date" value="${escapeHtml(transactionFromFilter)}" /></label>
      <label>Tot en met <input name="to" type="date" value="${escapeHtml(transactionToFilter)}" /></label>
      <label>Zoeken <input name="search" value="${escapeHtml(transactionSearchFilter)}" placeholder="Omschrijving of tegenpartij" /></label>
      <div class="core-actions"><button type="submit">${escapeHtml(t('action.applyFilter', 'Filter toepassen'))}</button><button type="button" class="secondary" data-action="reset-transaction-filters">${escapeHtml(t('action.reset', 'Reset'))}</button></div>
    </form>
    <div class="core-stats" aria-label="Totalen van de geselecteerde transacties">
      <div><strong>${money.format(transactionTotals.incomeCents / 100)}</strong><span>Inkomsten</span></div>
      <div><strong>${money.format(-transactionTotals.expenseCents / 100)}</strong><span>Uitgaven</span></div>
      <div><strong>${money.format(transactionTotals.balanceCents / 100)}</strong><span>Saldo van de selectie</span></div>
    </div>
    ${transactions.length ? `<form id="bulk-classification-form" class="transaction-bulk-bar"><strong>${selectedTransactionIDs.size} geselecteerd</strong><label>Categorie <select name="financialObjectId" required><option value="">Kies een financieel object</option>${objects.map((object) => `<option value="${object.id}">${escapeHtml(object.name)} (${escapeHtml(typeLabel(object.objectType))})</option>`).join('')}</select></label><button type="submit" ${selectedTransactionIDs.size && objects.length ? '' : 'disabled'}>${escapeHtml(t('action.changeCategory', 'Categorie wijzigen'))}</button><button type="button" class="secondary" data-action="clear-transaction-selection" ${selectedTransactionIDs.size ? '' : 'disabled'}>${escapeHtml(t('action.clearSelection', 'Selectie wissen'))}</button><small>Vervangt alle huidige toewijzingen van de geselecteerde transacties.</small></form>
    <div class="core-table-scroll"><table class="core-transaction-table"><thead><tr><th class="transaction-selection-column"><label><input type="checkbox" data-select-all-transactions aria-label="Alle zichtbare transacties selecteren" ${transactions.filter((item) => !item.hidden).length && transactions.filter((item) => !item.hidden).every((item) => selectedTransactionIDs.has(item.id)) ? 'checked' : ''} ${transactions.some((item) => !item.hidden) ? '' : 'disabled'} /> Selecteer</label></th><th>${renderSortButton('date', 'Datum')}</th><th>${renderSortButton('description', 'Omschrijving / tegenpartij')}</th><th>${renderSortButton('amount', 'Bedrag')}</th><th>Status</th><th>Objecten / toewijzing</th><th>Acties</th></tr></thead><tbody>${transactions.map((item) => `<tr>
      <td><input type="checkbox" data-transaction-selection data-id="${item.id}" aria-label="Selecteer transactie van ${escapeHtml(item.transactionDate)}" ${selectedTransactionIDs.has(item.id) ? 'checked' : ''} ${item.hidden ? 'disabled' : ''} /></td>
      <td>${escapeHtml(item.transactionDate)}</td>
      <td><strong>${escapeHtml(item.description || 'Geen omschrijving')}</strong><small>${escapeHtml(item.counterparty)}</small></td>
      <td class="${item.amountCents > 0 ? 'income' : 'expense'}">${money.format(item.amountCents / 100)}<small>${item.amountCents > 0 ? escapeHtml(t('transaction.direction.income', 'Inkomst')) : escapeHtml(t('transaction.direction.expense', 'Uitgave'))}</small></td>
      <td><span class="config-status ${item.status === 'A' || item.status === 'U' ? 'ok' : 'warning'}">${escapeHtml(transactionStatuses[item.status] || item.status || 'Onbekend')}</span></td>
      <td>${item.hidden ? '<span class="muted">Verborgen</span>' : (item.allocations.length ? `<ul class="allocation-labels">${item.allocations.map((link) => `<li>${escapeHtml(link.name)} · ${link.amountCents == null ? 'context' : money.format(link.amountCents / 100)}</li>`).join('')}</ul>` : '<span class="muted">Nog niet geclassificeerd</span>')}<small>${item.fullyAllocated ? 'Volledig toegewezen' : `Nog toe te wijzen: ${money.format(item.unallocatedAmountCents / 100)}`}</small></td>
      <td><div class="core-actions"><button type="button" class="secondary" data-action="edit-transaction" data-id="${item.id}" ${item.hidden ? 'disabled' : ''}>${escapeHtml(t('action.edit', 'Wijzigen'))}</button>${item.hidden ? `<button type="button" class="secondary" data-action="unhide-transaction" data-id="${item.id}">${escapeHtml(t('action.unhide', 'Zichtbaar maken'))}</button>` : `<button type="button" class="secondary" data-action="delete-transaction" data-id="${item.id}">${escapeHtml(t('action.hide', 'Verbergen'))}</button>`}</div></td>
    </tr>`).join('')}</tbody></table></div>` : '<p class="muted">Nog geen transacties in deze selectie.</p>'}
    <div class="core-pagination"><button type="button" class="secondary" data-action="previous-transactions" ${transactionOffset === 0 ? 'disabled' : ''}>${escapeHtml(t('action.previous', 'Vorige'))}</button><span>${transactionTotals.total ? `${transactionOffset + 1}–${Math.min(transactionOffset + transactions.length, transactionTotals.total)} van ${transactionTotals.total}` : '0 transacties'}</span><button type="button" class="secondary" data-action="next-transactions" ${transactionOffset + transactionPageSize >= transactionTotals.total ? 'disabled' : ''}>${escapeHtml(t('action.next', 'Volgende'))}</button></div>
    <section class="core-detail"><h3>${draft ? 'Transactie wijzigen' : 'Transactie aanmaken'}</h3>
      <form id="transaction-form">
        <div class="transaction-fields">
          <label>Soort <select name="direction"><option value="expense" ${direction === 'expense' ? 'selected' : ''}>${escapeHtml(t('transaction.direction.expense', 'Uitgave'))}</option><option value="income" ${direction === 'income' ? 'selected' : ''}>${escapeHtml(t('transaction.direction.income', 'Inkomst'))}</option></select></label>
          <label>Datum <input name="transactionDate" type="date" value="${escapeHtml(draft?.transactionDate || new Date().toLocaleDateString('sv-SE'))}" required /></label>
          <label>Bedrag (€) <input name="amount" type="text" inputmode="decimal" value="${amountInput(draft?.amountCents)}" placeholder="83,42" required /></label>
          <label>Tegenpartij <input name="counterparty" value="${escapeHtml(draft?.counterparty)}" /></label>
        </div>
        <label>Omschrijving <input name="description" value="${escapeHtml(draft?.description)}" /></label>
        <p class="muted">Vul bedragen positief in; de gekozen soort bepaalt het teken. Splits een bedrag over meerdere objecten of laat het nog ongeclassificeerd. Waardehistorie wordt afzonderlijk onderhouden.</p>
        <div id="allocation-rows">${rows.map(renderAllocationRow).join('')}</div>
        <button type="button" class="secondary" data-action="add-allocation" ${objects.length ? '' : 'disabled'}>${escapeHtml(t('action.addAllocation', 'Object koppelen / splitsing toevoegen'))}</button>
        ${draft ? '' : `<fieldset class="recurring-transaction-fields"><legend>Maandelijkse transacties</legend><label class="checkbox-label"><input name="repeatMonthly" type="checkbox" /> Maandelijks herhalen</label><label>Einddatum <input name="repeatEndDate" type="date" /></label><p class="muted">Maakt direct losse transacties aan voor elke maand tot en met de einddatum.</p></fieldset>`}
        <label><input name="applyToMatching" type="checkbox" checked /> Zelfde categorisatie toepassen op zichtbare transacties met dezelfde tegenpartij die nog niet geclassificeerd zijn</label>
        <div class="core-actions"><button type="submit">${draft ? escapeHtml(t('action.saveTransaction', 'Transactie opslaan')) : escapeHtml(t('action.createTransaction', 'Transactie aanmaken'))}</button>${draft ? `<button type="button" class="secondary" data-action="new-transaction">${escapeHtml(t('action.cancel', 'Annuleren'))}</button>` : ''}</div>
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
  if (event.target.matches('[data-document-select]')) {
    const id = Number(event.target.dataset.documentSelect);
    if (event.target.checked) selectedDocumentIDs.add(id);
    else selectedDocumentIDs.delete(id);
    updateDocuments();
    return;
  }
  if (event.target.matches('[data-transaction-selection]')) {
    const id = Number(event.target.dataset.id);
    if (event.target.checked) selectedTransactionIDs.add(id);
    else selectedTransactionIDs.delete(id);
    render();
    return;
  }
  if (event.target.matches('[data-select-all-transactions]')) {
    for (const transaction of transactions) {
      if (transaction.hidden) continue;
      if (event.target.checked) selectedTransactionIDs.add(transaction.id);
      else selectedTransactionIDs.delete(transaction.id);
    }
    render();
    return;
  }
  if (event.target.closest('#transaction-filter-form') && event.target.name === 'view') {
    transactionViewFilter = event.target.value;
    transactionOffset = 0;
    refresh().catch(showError);
  }
  if (event.target.closest('#ai-runtime-config-form') && event.target.name === 'transactionAiUrl') {
    loadTransactionAIModels(event.target.value).catch(showError);
  } else if (event.target.closest('#ai-runtime-config-form') && ['reuseDocumentAI', 'pdfAiUrl', 'pdfAiModel'].includes(event.target.name)) {
    updateAIModelControls();
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
  if (event.target.matches('[data-document-filter]')) {
    const position = event.target.selectionStart;
    documentSearchFilter = event.target.value;
    updateDocuments();
    const input = document.querySelector('[data-document-filter]');
    input?.focus();
    if (input && position != null) input.setSelectionRange(position, position);
    return;
  }
  if (event.target.closest('#ai-runtime-config-form') && ['pdfAiUrl', 'pdfAiModel'].includes(event.target.name)) {
    documentAIModels = [];
    documentAIModelsError = '';
    if (event.target.closest('#ai-runtime-config-form').elements.reuseDocumentAI.checked) {
      transactionAIModels = [];
      transactionAIModelsError = '';
    }
    updateAIModelControls();
  } else if (event.target.closest('#ai-runtime-config-form') && ['transactionAiUrl', 'reuseDocumentAI'].includes(event.target.name)) {
    transactionAIModels = [];
    transactionAIModelsError = '';
    updateAIModelControls();
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
  if (action === 'budget-year') {
    budgetYear = Number(button.dataset.year);
    budgetMonth = 0;
    await loadBudget();
    return;
  }
  if (action === 'budget-month') {
    const month = Number(button.dataset.month);
    budgetMonth = budgetMonth === month ? 0 : month;
    await loadBudget();
    return;
  }
  if (action === 'configuration-section') {
    configurationFocus = button.dataset.section;
    render();
    return;
  }
  if (action === 'restart-application') {
    if (!confirm('Applicatie nu herstarten? De pagina is kort niet bereikbaar.')) return;
    button.disabled = true;
    try {
      await api('/system/restart', { method: 'POST' });
      message = 'Applicatie wordt herstart. Vernieuw de pagina over enkele seconden.';
      render();
    } catch (error) {
      showError(error);
      button.disabled = false;
    }
    return;
  }
  if (action === 'load-transaction-ai-models') {
    const form = document.querySelector('#ai-runtime-config-form');
    const url = form?.elements?.transactionAiUrl?.value || '';
    await loadTransactionAIModels(url);
    return;
  }
  if (action === 'load-document-ai-models') {
    const form = document.querySelector('#ai-runtime-config-form');
    const url = form?.elements?.pdfAiUrl?.value || '';
    await loadDocumentAIModels(url);
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
      bankImportMessage = `${result.imported} transacties geïmporteerd; ${result.skipped || 0} duplicaten overgeslagen.`;
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
  if (action === 'clear-transaction-selection') {
    selectedTransactionIDs.clear();
    render();
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
  if (action === 'close-document-upload') {
    documentUploadOpen = false;
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
      objectTransactionsTotal = 0;
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
      objectTransactionsTotal = 0;
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
    } else if (action === 'batch-analyze-documents') {
      const result = await api('/documents/analyze-batch', { method: 'POST' });
      documentMessage = `${result.queued || 0} ${tt('documenten aangeboden voor AI-analyse.')}`;
      await reloadDocuments();
    } else if (action === 'analyze-selected-documents') {
      const result = await api('/documents/analyze-selected', { method: 'POST', body: JSON.stringify({ documentIds: [...selectedDocumentIDs] }) });
      selectedDocumentIDs.clear();
      documentMessage = `${result.queued || 0} geselecteerde documenten aangeboden voor AI-analyse.`;
      await reloadDocuments();
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
    if (form.matches('.transaction-reassign-form')) {
      await api('/transactions/bulk-classify', {
        method: 'POST', body: JSON.stringify({ transactionIds: [Number(form.dataset.transactionId)], financialObjectId: Number(data.financialObjectId) }),
      });
    } else if (form.id === 'object-form') {
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
      const amountCents = sign * positiveCents(data.amount);
      const repeatMonthly = !transactionDraft && form.elements.repeatMonthly?.checked;
      const dates = repeatMonthly ? monthlyTransactionDates(data.transactionDate, data.repeatEndDate) : [data.transactionDate];
      for (const transactionDate of dates) {
        await api(transactionDraft ? `/transactions/${transactionDraft.id}` : '/transactions', {
          method: transactionDraft ? 'PUT' : 'POST',
          body: JSON.stringify({ transactionDate, amountCents, description: data.description, counterparty: data.counterparty, allocations, applyToMatching: !repeatMonthly && form.elements.applyToMatching.checked }),
        });
      }
      if (repeatMonthly) message = `${dates.length} ${tt('maandelijkse transacties aangemaakt.')}`;
      ++transactionEditRequestID;
      transactionDraft = null;
      transactionOffset = 0;
    } else if (form.id === 'bulk-classification-form') {
      if (!selectedTransactionIDs.size) throw new Error('Selecteer minstens één transactie.');
      await api('/transactions/bulk-classify', {
        method: 'POST', body: JSON.stringify({ transactionIds: [...selectedTransactionIDs], financialObjectId: Number(data.financialObjectId) }),
      });
      selectedTransactionIDs.clear();
    } else if (form.id === 'configuration-form') {
      const accessKey = data.s3AccessKey.trim();
      const secretKey = data.s3SecretKey.trim();
      const databaseDirectory = configuration.database.directory || '/app/data';
      const payload = {
        databasePath: `${databaseDirectory}/${data.databaseFile || configuration.database.filename || 'hohoho.db'}`,
        s3Endpoint: data.s3Endpoint.trim(), s3Bucket: data.s3Bucket.trim(), s3Region: data.s3Region.trim(), pdfAiUrl: configuration.runtime.pdfAiUrl || '', pdfAiModel: configuration.runtime.pdfAiModel || 'Qwen3.8-Flash-Next-UD-Q4_K_XL', transactionAiUrl: configuration.runtime.transactionAiUrl || '', transactionAiModel: configuration.runtime.transactionAiModel || transactionAIModels[0] || defaultTransactionAIModel, language: configuration.runtime.language || uiLanguage,
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
          databasePath: configuration.database.path || '/app/data/hohoho.db',
          s3Endpoint: storage.endpoint || '', s3Bucket: storage.bucket || '', s3Region: storage.region || 'garage',
          s3UseTLS: Boolean(storage.tls), s3PathStyle: Boolean(storage.pathStyle), s3CreateBucket: Boolean(storage.createBucket),
          pdfAiUrl: configuration.runtime.pdfAiUrl || '', pdfAiModel: configuration.runtime.pdfAiModel || 'Qwen3.8-Flash-Next-UD-Q4_K_XL', transactionAiUrl: configuration.runtime.transactionAiUrl || '', transactionAiModel: configuration.runtime.transactionAiModel || transactionAIModels[0] || defaultTransactionAIModel, language: data.language || configuration.runtime.language || uiLanguage,
        }),
      });
      const translations = await loadTranslations(configuration.runtime.language);
      if (translations) {
        uiLanguage = translations.language || uiLanguage;
        uiTranslations = translations.messages || {};
      }
    } else if (form.id === 'ai-runtime-config-form') {
      const storage = configuration.objectStorage;
      const documentURL = data.pdfAiUrl.trim();
      const documentModel = data.pdfAiModel.trim() || 'Qwen3.8-Flash-Next-UD-Q4_K_XL';
      const reuseDocumentAI = form.elements.reuseDocumentAI.checked;
      configuration = await api('/system/configuration', {
        method: 'PUT',
        body: JSON.stringify({
          databasePath: configuration.database.path || '/app/data/hohoho.db',
          s3Endpoint: storage.endpoint || '', s3Bucket: storage.bucket || '', s3Region: storage.region || 'garage',
          s3UseTLS: Boolean(storage.tls), s3PathStyle: Boolean(storage.pathStyle), s3CreateBucket: Boolean(storage.createBucket),
          pdfAiUrl: documentURL, pdfAiModel: documentModel, transactionAiUrl: reuseDocumentAI ? documentURL : data.transactionAiUrl.trim(), transactionAiModel: reuseDocumentAI ? documentModel : (data.transactionAiModel || transactionAIModels[0] || defaultTransactionAIModel), language: configuration.runtime.language || uiLanguage,
        }),
      });
    } else if (form.id === 'backup-form') {
      const rcloneConfig = data.rcloneConfig.trim();
      const resticPassword = data.resticPassword;
      const remoteName = data.remoteName.trim();
      const bucket = data.bucket.trim().replace(/^\/+|\/+$/g, '');
      const backupFolder = data.backupFolder.trim().replace(/^\/+|\/+$/g, '');
      const endpoint = data.s3Endpoint.trim();
      const accessKey = data.s3AccessKey.trim();
      const secretKey = data.s3SecretKey.trim();
      if (!rcloneConfig && (endpoint || accessKey || secretKey)) {
        if (!endpoint || !accessKey || !secretKey) throw new Error('Vul endpoint, access key en secret key volledig in om de opslagverbinding te wijzigen.');
      } else if (!rcloneConfig && !backupStatus?.rcloneConfigSet) {
        throw new Error('Vul de gegevens van de opslagverbinding in of gebruik een eigen rclone-configuratie.');
      }
      const payload = { repository: `${remoteName}:${bucket}/${backupFolder}`, policy: { schedule: data.schedule, keepDaily: Number(data.keepDaily), keepWeekly: Number(data.keepWeekly), keepMonthly: Number(data.keepMonthly) } };
      if (rcloneConfig) payload.rcloneConfig = rcloneConfig;
      else if (endpoint) payload.rcloneConfig = `[${remoteName}]\ntype = s3\nprovider = Other\naccess_key_id = ${accessKey}\nsecret_access_key = ${secretKey}\nendpoint = ${endpoint}\nforce_path_style = true`;
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
    } else if (form.id === 'ai-docs-config-form') {
      aiDocsConfig = await api('/system/ai-docs-config', { method: 'PUT', body: JSON.stringify({ rawYaml: data.rawYaml }) });
      aiDocsConfigMessage = 'Document-AI configuratie is opgeslagen.';
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
