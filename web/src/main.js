import './style.css';

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || '';
const app = document.querySelector('#app');
let bankProfiles = ['generic'];
let mainCategoriesState = [];
let categoriesState = [];
let accountsState = [];
let transactionYearsState = [];
let transactionFilterYear = '';
let transactionFilterMonth = '';
let dashboardFilterYear = '';
let dashboardFilterMonth = '';
let dashboardTransactionSelection = null;
let accountTransactionsUncategorizedOnly = false;
let accountTransactionPage = { offset: 0, limit: 25 };
let categoryTransactionPage = { offset: 0, limit: 25 };
let assetsState = [];
let assetListState = [];
let selectedAssetState = null;
let selectedSavingsAccountState = null;
let assetValuationsState = [];
let assetContractsState = [];
let assetsLoaded = false;
let assetDetailRequestID = 0;
let liabilitiesState = [];
let selectedLiabilityState = null;
let liabilityContractsState = [];
let liabilitiesLoaded = false;
let liabilityDetailRequestID = 0;
let reservationsState = [];
let selectedReservationState = null;
let reservationsLoaded = false;
let reservationDetailRequestID = 0;
let casesState = [];
let selectedCaseState = null;
let caseDocumentsState = [];
let caseTransactionsState = [];
let caseTransactionCandidatesState = [];
let caseRelationsState = [];
let caseAssetsState = [];
let caseLiabilitiesState = [];
let casesLoaded = false;
let caseDetailRequestID = 0;
let caseListRequestID = 0;
let caseTransactionRequestID = 0;
let systemConfigurationLoaded = false;
let systemConfigurationLoading = false;
let backupConfigurationLoaded = false;
let backupConfigurationLoading = false;
let backupPollTimer = null;

async function loadAccounts() {
  const response = await fetch(`${apiBaseUrl}/api/v1/accounts`);
  if (!response.ok) {
    throw new Error('Bankrekeningen konden niet worden geladen');
  }
  return response.json();
}

async function loadMainCategories() {
  const response = await fetch(`${apiBaseUrl}/api/v1/main-categories`);
  if (!response.ok) {
    throw new Error('Hoofdcategorieen konden niet worden geladen');
  }
  return response.json();
}

async function loadSummary() {
  const response = await fetch(`${apiBaseUrl}/api/v1/summary`);
  if (!response.ok) {
    throw new Error('Samenvatting kon niet worden geladen');
  }
  return response.json();
}

async function loadCategories() {
  const response = await fetch(`${apiBaseUrl}/api/v1/categories`);
  if (!response.ok) {
    throw new Error('Categorieen konden niet worden geladen');
  }
  return response.json();
}

async function loadTransactionYears() {
  const response = await fetch(`${apiBaseUrl}/api/v1/transactions/years`);
  if (!response.ok) {
    throw new Error('Transactiejaren konden niet worden geladen');
  }
  return response.json();
}

function categoriesFor(categories, purpose) {
  return categories
    .filter((category) => category.purpose === purpose)
    .map((category) => `<li><span>${category.name}</span><small>${category.kind}</small></li>`)
    .join('');
}

function categoryOptionsFor(kind) {
  const groups = {
    income: new Set(['Inkomsten']),
    fixed: new Set(['Wonen', 'Verzekeringen', 'Telecom']),
    variable: new Set(['Huishouden', 'Medische kosten', 'Vervoer', 'Educatie', 'Kleding', 'Vrije tijd', 'Overige uitgaven']),
    investment: new Set(['Wonen']),
  };
  const mainIDs = mainCategoriesState.filter((category) => groups[kind]?.has(category.name)).map((category) => category.id);
  const options = categoriesState.filter((category) => {
    if (kind === 'income') {
      return category.kind === 'income';
    }
    if (kind === 'investment') {
      return category.purpose === 'investment';
    }
    return mainIDs.includes(category.mainCategoryId);
  });
  if (!options.length) {
    return '<option value="">Geen categorieen</option>';
  }
  return options.map((category) => `<option value="${category.id}">${escapeHtml(category.name)}</option>`).join('');
}

function mainCategoryOptions() {
  return mainCategoriesState.map((category) => `<option value="${category.id}">${escapeHtml(category.name)}</option>`).join('');
}

function subcategoryOptions(mainCategoryID) {
  const categories = categoriesState.filter((category) => category.mainCategoryId === Number(mainCategoryID));
  if (!categories.length) {
    return '<option value="">Alle subcategorieen</option>';
  }
  return '<option value="">Alle subcategorieen</option>' + categories.map((category) => `<option value="${category.id}">${escapeHtml(category.name)}</option>`).join('');
}

function render({ summary, mainCategories, categories, accounts, transactionYears }) {
  mainCategoriesState = mainCategories;
  categoriesState = categories;
  accountsState = accounts;
  transactionYearsState = transactionYears;
  transactionFilterYear = defaultTransactionYear();
  dashboardFilterYear = defaultTransactionYear();

  app.innerHTML = `
    <section class="shell">
      <nav class="main-menu" aria-label="Hoofdmenu">
        <button type="button" class="main-tab active" data-main-target="dashboard">Dashboard</button>
        <button type="button" class="main-tab" data-main-target="transactions">Inkomsten &amp; uitgaven</button>
        <button type="button" class="main-tab" data-main-target="wealth">Vermogen</button>
        <button type="button" class="main-tab" data-main-target="configuration">Configuratie</button>
      </nav>

      <section class="app-tab-section active" data-main-section="dashboard" aria-label="Dashboard">
        <div class="dashboard-grid">
          <details class="dashboard-panel" open>
            <summary><span>Grafieken</span><strong>Uitgaven</strong></summary>
            <form id="chart-filter-form" class="chart-filter-form">
              <div class="period-filter">
                <span class="period-filter-label">Jaar</span>
                <div id="dashboard-year-buttons" class="period-buttons" role="group" aria-label="Dashboardjaar">${dashboardYearButtons()}</div>
                <input name="year" type="hidden" value="${dashboardFilterYear}" />
              </div>
              <div class="period-filter">
                <span class="period-filter-label">Maand</span>
                <div id="dashboard-month-buttons" class="period-buttons month-buttons" role="group" aria-label="Dashboardmaand">${dashboardMonthButtons()}</div>
                <input name="month" type="hidden" value="" />
              </div>
              <label>
                Inzoomen op hoofdcategorie
                <select name="mainCategoryId">${chartMainCategoryOptions()}</select>
              </label>
            </form>
            <div id="chart-result" class="chart-placeholder">Kies een periode voor uitgaven per hoofdcategorie.</div>
          </details>
          <div class="dashboard-side-column">
            <details class="dashboard-panel" open>
              <summary><span>Vermogen</span><strong id="dashboard-net-worth">Laden...</strong></summary>
              <div id="dashboard-wealth-summary" class="dashboard-wealth-summary" aria-live="polite">
                <p>Bezittingen en schulden laden...</p>
              </div>
            </details>
            <details class="dashboard-panel" open>
              <summary><span>Contracten</span><strong id="dashboard-contract-count">Laden...</strong></summary>
              <div id="dashboard-contracts" class="dashboard-contracts" aria-live="polite">
                <p>Contracten laden...</p>
              </div>
            </details>
          </div>
        </div>
        <section class="dashboard-category-transactions transaction-panel" aria-label="Dashboardtransacties">
          <div class="panel-heading">
            <h2>Transacties</h2>
            <span id="dashboard-transaction-label">Kies een grafiekrubriek.</span>
          </div>
          <div id="dashboard-transaction-result" class="table-wrap"><p>Kies een grafiekrubriek.</p></div>
        </section>
      </section>

      <section class="app-tab-section" data-main-section="transactions" aria-label="Inkomsten en uitgaven">
        <section class="transaction-panel" aria-label="Transactiekeuzes">
          <div class="transaction-filter-card">
            <div class="transaction-filter-row full-width">
              <label>Zoeken <input id="account-transaction-search" type="search" placeholder="Omschrijving, tegenpartij, IBAN of referentie" /></label>
            </div>
            <form id="category-transaction-filter-form" class="category-transaction-filter-form">
              <label>
                Hoofdcategorie
                <select name="mainCategoryId">${mainCategoryOptionsWithAll()}</select>
              </label>
              <label>
                Subcategorie
                <select name="categoryId" disabled><option value="">Kies eerst een hoofdcategorie</option></select>
              </label>
            </form>
            <form id="transaction-filter-form" class="transaction-filter-form">
              <div class="transaction-filter-row period-filter-rows">
                <div class="period-filter period-filter-years">
                  <span class="period-filter-label">Jaar</span>
                  <div id="transaction-year-buttons" class="period-buttons" role="group" aria-label="Jaar">${transactionYearButtons()}</div>
                  <input name="year" type="hidden" value="${transactionFilterYear}" />
                </div>
                <div class="period-filter period-filter-months">
                  <span class="period-filter-label">Maand</span>
                  <div id="transaction-month-buttons" class="period-buttons month-buttons" role="group" aria-label="Maand">${transactionMonthButtons()}</div>
                  <input name="month" type="hidden" value="" />
                </div>
              </div>
              <div class="transaction-filter-row two-columns">
                <label>
                  Rekening
                  <select id="dashboard-account-select">
                    ${renderTransactionAccountOptions()}
                  </select>
                </label>
                <label>Aantal regels
                  <select name="limit">
                    <option value="10">10</option>
                    <option value="25" selected>25</option>
                    <option value="50">50</option>
                    <option value="100">100</option>
                  </select>
                </label>
              </div>
            </form>
          </div>
          <div class="transaction-count"><span>Transacties: <strong id="shown-transaction-count">0</strong></span><span>Totaal: <strong id="shown-transaction-total">${formatAmount(0)}</strong></span></div>
        </section>
        <details class="transaction-panel" aria-label="Transacties per rekening" open>
          <summary><span>Transacties</span><strong>Open</strong></summary>
          <div id="transaction-result" class="table-wrap"><p>Kies een rekening en periode.</p></div>
        </details>
        <details class="transaction-panel manual-flow-panel">
          <summary><span>Handmatige geldstroom</span><strong>Toevoegen</strong></summary>
          <p class="manual-flow-explanation">Gebruik voor contante betalingen een aparte rekening, bijvoorbeeld "Kas". Zo blijven het kassaldo en alle geldstromen controleerbaar.</p>
          <form id="manual-transaction-form" class="manual-transaction-form">
            <label>Soort <select name="direction"><option value="expense">Uitgave</option><option value="income">Inkomst</option></select></label>
            <label>Rekening <select name="accountId" required>${renderManualAccountOptions()}</select></label>
            <label>Datum <input name="bookedOn" type="date" value="${localDateValue()}" required /></label>
            <label>Bedrag <input name="amount" type="number" min="0.01" step="0.01" required /></label>
            <label>Subcategorie <select name="categoryId">${manualTransactionCategoryOptions('expense')}</select></label>
            <label>Omschrijving <input name="description" type="text" required /></label>
            <label>Tegenpartij <input name="counterparty" type="text" /></label>
            <button type="submit">Geldstroom toevoegen</button>
          </form>
          <p id="manual-transaction-status" class="document-status" aria-live="polite"></p>
        </details>
        <section class="import-panel" aria-label="Bankimport">
          <div>
            <p class="eyebrow">Bankimport</p>
            <h2>Importpreview</h2>
            <p>Upload een CSV-export of een ICS PDF-afschrift. Controleer de preview voordat je definitief importeert.</p>
          </div>
          <form id="import-form">
            <label>
              Bankrekening
              <select name="accountId" ${accountsState.length ? '' : 'disabled'}>
                ${renderImportAccountOptions()}
              </select>
            </label>
            <label>
              Importbestand
              <input name="file" type="file" accept=".csv,.pdf,text/csv,application/pdf" required />
            </label>
            <div class="form-actions">
              <button type="submit" data-import-action="preview">Preview import</button>
              <button type="submit" class="secondary-button" data-import-action="commit">Definitief importeren</button>
            </div>
          </form>
          <div id="import-result" class="import-result"></div>
        </section>
      </section>

      <section class="app-tab-section" data-main-section="wealth" aria-label="Vermogen">
        <nav class="wealth-tabs" aria-label="Vermogen onderdelen" role="tablist">
          <button type="button" class="wealth-tab active" data-wealth-section-target="assets" role="tab" aria-selected="true">Bezittingen</button>
          <button type="button" class="wealth-tab" data-wealth-section-target="liabilities" role="tab" aria-selected="false">Schulden</button>
          <button type="button" class="wealth-tab" data-wealth-section-target="reservations" role="tab" aria-selected="false">Reserveringen</button>
        </nav>
        <section class="wealth-section active" data-wealth-section="assets">
          <section id="assets-root" class="assets-shell">
          <header class="assets-heading">
            <div><p class="eyebrow">Vermogen</p><h1>Bezittingen</h1><p>Beheer bezittingen en hun documenten en transacties.</p></div>
            <button type="button" class="asset-new-button" data-asset-action="new">Nieuwe bezitting</button>
          </header>
          <form id="asset-filter-form" class="asset-filter-form">
            <label>Type <select name="type"><option value="">Alle typen</option>${assetTypeOptions()}</select></label>
            <label>Status <select name="status"><option value="">Alle statussen</option>${assetStatusOptions()}</select></label>
          </form>
          <div class="assets-layout">
            <aside class="asset-list-panel"><div id="asset-list"><p>Open Bezittingen om de lijst te laden.</p></div></aside>
            <div class="asset-content">
              <section id="asset-form-panel" class="asset-panel hidden">
                <div class="panel-heading"><h2 id="asset-form-title">Nieuwe bezitting</h2></div>
                <form id="asset-form" class="asset-form">
                  <input name="id" type="hidden" />
                  <label>Naam <input name="name" type="text" required /></label>
                  <label>Type <select name="assetType" required>${assetTypeOptions()}</select></label>
                  <label>Status <select name="status" required>${assetStatusOptions('active')}</select></label>
                  <label>Aankoopdatum <input name="acquiredAt" type="date" /></label>
                  <label>Afvoerdatum <input name="disposedAt" type="date" /></label>
                  <label>Aankoopwaarde <input name="purchaseValue" type="number" min="0" step="0.01" value="0.00" /></label>
                  <label class="asset-description-field">Omschrijving <textarea name="description" rows="3"></textarea></label>
                  <div class="form-actions asset-form-actions"><button type="submit">Opslaan</button><button type="button" class="secondary-button" data-asset-action="cancel-form">Annuleren</button></div>
                </form>
              </section>
              <section id="asset-detail" class="asset-panel hidden">
                <div class="asset-detail-heading"><div><small>Bezitting</small><h2 id="asset-detail-name"></h2></div><div class="row-actions"><button type="button" class="secondary-button" data-asset-action="edit">Wijzigen</button><button type="button" class="secondary-button" data-asset-action="delete">Verwijderen</button></div></div>
                <nav class="asset-detail-tabs" aria-label="Bezitting onderdelen">
                  <button type="button" class="asset-detail-tab active" data-asset-detail-target="overview">Overzicht</button>
                  <button type="button" class="asset-detail-tab" data-asset-detail-target="value">Waarde</button>
                  <button type="button" class="asset-detail-tab" data-asset-detail-target="contracts">Contracten</button>
                  <button type="button" class="asset-detail-tab" data-asset-detail-target="documents">Documenten</button>
                  <button type="button" class="asset-detail-tab" data-asset-detail-target="transactions">Transacties</button>
                  <button type="button" class="asset-detail-tab" data-asset-detail-target="timeline">Tijdlijn</button>
                </nav>
                <section class="asset-detail-section active" data-asset-detail-section="overview" id="asset-overview"></section>
                <section class="asset-detail-section" data-asset-detail-section="value">
                  <div class="asset-current-value"><small>Actuele waarde uit meest recente waardering</small><strong id="asset-current-value"></strong></div>
                  <form id="asset-valuation-form" class="asset-valuation-form">
                    <input name="id" type="hidden" />
                    <label>Datum <input name="valuationDate" type="date" required /></label>
                    <label>Waarde <input name="value" type="number" min="0" step="0.01" required /></label>
                    <label>Type <select name="valuationType" required>${assetValuationTypeOptions()}</select></label>
                    <label>Bron <input name="source" type="text" /></label>
                    <label class="asset-valuation-notes">Notitie <textarea name="notes" rows="2"></textarea></label>
                    <div class="form-actions asset-valuation-actions"><button type="submit">Waardering opslaan</button><button type="button" class="secondary-button hidden" data-asset-action="cancel-valuation-edit">Annuleren</button></div>
                  </form>
                  <div id="asset-valuation-list" class="asset-valuation-list"></div>
                </section>
                <section class="asset-detail-section" data-asset-detail-section="contracts">
                  <form id="asset-contract-form" class="asset-form case-form">
                    <input name="id" type="hidden" />
                    <label>Titel <input name="title" type="text" required /></label>
                    <label>Startdatum <input name="startDate" type="date" /></label>
                    <label>Einddatum <input name="endDate" type="date" /></label>
                    <label>Looptijd in maanden <input name="termMonths" type="number" min="1" step="1" inputmode="numeric" /></label>
                    <label>Status <select name="status" required>${caseStatusOptions('active')}</select></label>
                    <label class="asset-description-field">Omschrijving <textarea name="description" rows="3"></textarea></label>
                    <div class="form-actions asset-form-actions"><button type="submit">Contract opslaan</button><button type="button" class="secondary-button hidden" data-asset-action="cancel-contract-edit">Annuleren</button></div>
                  </form>
                  <div id="asset-contract-list" class="asset-related-list"></div>
                </section>
                <section class="asset-detail-section" data-asset-detail-section="documents">
                  <div id="asset-document-list" class="document-list"></div>
                  <form id="asset-document-upload-form" class="document-form">
                    <label>Bestand <input name="file" type="file" required /></label><label>Documentdatum <input name="documentDate" type="date" /></label><label>Omschrijving <input name="description" type="text" /></label><button type="submit">Uploaden en koppelen</button>
                  </form>
                  <form id="asset-document-link-form" class="document-link-form"><label>Bestaand document <select name="documentId" required></select></label><button type="submit" class="secondary-button">Bestaand document koppelen</button></form>
                  <p id="asset-document-status" class="document-status" aria-live="polite"></p>
                </section>
                <section class="asset-detail-section" data-asset-detail-section="transactions">
                  <form id="asset-transaction-link-form" class="asset-transaction-link-form"><label>Zoek transactie <input name="search" type="search" placeholder="Omschrijving of tegenpartij" /></label><button type="button" class="secondary-button" data-asset-action="search-transactions">Zoeken</button><label>Transactie <select name="transactionId" required></select></label><button type="submit">Transactie koppelen</button></form>
                  <div id="asset-transaction-list" class="asset-related-list"></div>
                </section>
                <section class="asset-detail-section" data-asset-detail-section="timeline"><div id="asset-timeline" class="domain-timeline"></div></section>
              </section>
              <section id="savings-account-detail" class="asset-panel hidden"></section>
              <section id="asset-empty-state" class="asset-panel empty-state"><strong>Kies een bezitting</strong><p>Selecteer een bezitting of maak een nieuwe aan.</p></section>
            </div>
          </div>
          </section>
        </section>
        <section class="wealth-section" data-wealth-section="liabilities">
          <section id="liabilities-root" class="assets-shell liabilities-shell">
          <header class="assets-heading">
            <div><p class="eyebrow">Verplichtingen</p><h1>Schulden</h1><p>Beheer schulden en hun contracten, documenten en transacties.</p></div>
            <button type="button" class="asset-new-button" data-liability-action="new">Nieuwe schuld</button>
          </header>
          <form id="liability-filter-form" class="asset-filter-form">
            <label>Type <select name="type"><option value="">Alle typen</option>${liabilityTypeOptions()}</select></label>
            <label>Status <select name="status"><option value="">Alle statussen</option>${liabilityStatusOptions('active')}</select></label>
          </form>
          <div class="assets-layout">
            <aside class="asset-list-panel"><div id="liability-list"><p>Open Schulden om de lijst te laden.</p></div></aside>
            <div class="asset-content">
              <section id="liability-form-panel" class="asset-panel hidden">
                <div class="panel-heading"><h2 id="liability-form-title">Nieuwe schuld</h2></div>
                <form id="liability-form" class="asset-form liability-form">
                  <input name="id" type="hidden" />
                  <label>Naam <input name="name" type="text" required /></label>
                  <label>Type <select name="liabilityType" required>${liabilityTypeOptions()}</select></label>
                  <label>Schuldeiser <input name="creditor" type="text" /></label>
                  <label>Status <select name="status" required>${liabilityStatusOptions('active')}</select></label>
                  <label>Startdatum <input name="startDate" type="date" /></label>
                  <label>Einddatum <input name="endDate" type="date" /></label>
                  <label>Oorspronkelijk bedrag <input name="originalAmount" type="number" min="0" step="0.01" value="0.00" /></label>
                  <label>Huidig saldo <input name="currentBalance" type="number" min="0" step="0.01" value="0.00" /></label>
                  <label class="asset-description-field">Omschrijving <textarea name="description" rows="3"></textarea></label>
                  <div class="form-actions asset-form-actions"><button type="submit">Opslaan</button><button type="button" class="secondary-button" data-liability-action="cancel-form">Annuleren</button></div>
                </form>
              </section>
              <section id="liability-detail" class="asset-panel hidden">
                <div class="asset-detail-heading"><div><small>Schuld</small><h2 id="liability-detail-name"></h2></div><div class="row-actions"><button type="button" class="secondary-button" data-liability-action="edit">Wijzigen</button><button type="button" class="secondary-button" data-liability-action="delete">Verwijderen</button></div></div>
                <nav class="asset-detail-tabs" aria-label="Schuld onderdelen">
                  <button type="button" class="asset-detail-tab active" data-liability-detail-target="overview">Overzicht</button>
                  <button type="button" class="asset-detail-tab" data-liability-detail-target="contracts">Contracten</button>
                  <button type="button" class="asset-detail-tab" data-liability-detail-target="documents">Documenten</button>
                  <button type="button" class="asset-detail-tab" data-liability-detail-target="transactions">Transacties</button>
                  <button type="button" class="asset-detail-tab" data-liability-detail-target="timeline">Tijdlijn</button>
                </nav>
                <section class="asset-detail-section active" data-liability-detail-section="overview" id="liability-overview"></section>
                <section class="asset-detail-section" data-liability-detail-section="contracts">
                  <form id="liability-contract-form" class="asset-form case-form">
                    <input name="id" type="hidden" />
                    <label>Titel <input name="title" type="text" required /></label>
                    <label>Startdatum <input name="startDate" type="date" /></label>
                    <label>Einddatum <input name="endDate" type="date" /></label>
                    <label>Looptijd in maanden <input name="termMonths" type="number" min="1" step="1" inputmode="numeric" /></label>
                    <label>Status <select name="status" required>${caseStatusOptions('active')}</select></label>
                    <label class="asset-description-field">Omschrijving <textarea name="description" rows="3"></textarea></label>
                    <div class="form-actions asset-form-actions"><button type="submit">Contract opslaan</button><button type="button" class="secondary-button hidden" data-liability-action="cancel-contract-edit">Annuleren</button></div>
                  </form>
                  <div id="liability-contract-list" class="asset-related-list"></div>
                </section>
                <section class="asset-detail-section" data-liability-detail-section="documents">
                  <div id="liability-document-list" class="document-list"></div>
                  <form id="liability-document-upload-form" class="document-form"><label>Bestand <input name="file" type="file" required /></label><label>Documentdatum <input name="documentDate" type="date" /></label><label>Omschrijving <input name="description" type="text" /></label><button type="submit">Uploaden en koppelen</button></form>
                  <form id="liability-document-link-form" class="document-link-form"><label>Bestaand document <select name="documentId" required></select></label><button type="submit" class="secondary-button">Bestaand document koppelen</button></form>
                  <p id="liability-document-status" class="document-status" aria-live="polite"></p>
                </section>
                <section class="asset-detail-section" data-liability-detail-section="transactions">
                  <form id="liability-transaction-link-form" class="asset-transaction-link-form"><label>Zoek transactie <input name="search" type="search" placeholder="Omschrijving of tegenpartij" /></label><button type="button" class="secondary-button" data-liability-action="search-transactions">Zoeken</button><label>Transactie <select name="transactionId" required></select></label><button type="submit">Transactie koppelen</button></form>
                  <div id="liability-transaction-list" class="asset-related-list"></div>
                </section>
                <section class="asset-detail-section" data-liability-detail-section="timeline"><div id="liability-timeline" class="domain-timeline"></div></section>
              </section>
              <section id="liability-empty-state" class="asset-panel empty-state"><strong>Kies een schuld</strong><p>Selecteer een schuld of maak een nieuwe aan.</p></section>
            </div>
          </div>
          </section>
        </section>
        <section class="wealth-section" data-wealth-section="reservations">
          <section id="reservations-root" class="assets-shell liabilities-shell">
          <header class="assets-heading">
            <div><p class="eyebrow">Toekomstige verplichtingen</p><h1>Reserveringen</h1><p>Leg toekomstige schulden of geplande verplichtingen vast met documenten en tijdlijn.</p></div>
            <button type="button" class="asset-new-button" data-reservation-action="new">Nieuwe reservering</button>
          </header>
          <form id="reservation-filter-form" class="asset-filter-form">
            <label>Type <select name="type"><option value="">Alle typen</option>${liabilityTypeOptions()}</select></label>
          </form>
          <div class="assets-layout">
            <aside class="asset-list-panel"><div id="reservation-list"><p>Open Reserveringen om de lijst te laden.</p></div></aside>
            <div class="asset-content">
              <section id="reservation-form-panel" class="asset-panel hidden">
                <div class="panel-heading"><h2 id="reservation-form-title">Nieuwe reservering</h2></div>
                <form id="reservation-form" class="asset-form liability-form">
                  <input name="id" type="hidden" />
                  <label>Naam <input name="name" type="text" required /></label>
                  <label>Type <select name="liabilityType" required>${liabilityTypeOptions()}</select></label>
                  <label>Verwachte datum <input name="endDate" type="date" /></label>
                  <label>Bedrag <input name="currentBalance" type="number" min="0" step="0.01" value="0.00" /></label>
                  <label class="asset-description-field">Omschrijving <textarea name="description" rows="3"></textarea></label>
                  <div class="form-actions asset-form-actions"><button type="submit">Opslaan</button><button type="button" class="secondary-button" data-reservation-action="cancel-form">Annuleren</button></div>
                </form>
              </section>
              <section id="reservation-detail" class="asset-panel hidden">
                <div class="asset-detail-heading"><div><small>Reservering</small><h2 id="reservation-detail-name"></h2></div><div class="row-actions"><button type="button" class="secondary-button" data-reservation-action="edit">Wijzigen</button><button type="button" class="secondary-button" data-reservation-action="delete">Verwijderen</button></div></div>
                <nav class="asset-detail-tabs" aria-label="Reservering onderdelen">
                  <button type="button" class="asset-detail-tab active" data-reservation-detail-target="overview">Overzicht</button>
                  <button type="button" class="asset-detail-tab" data-reservation-detail-target="documents">Documenten</button>
                  <button type="button" class="asset-detail-tab" data-reservation-detail-target="timeline">Tijdlijn</button>
                </nav>
                <section class="asset-detail-section active" data-reservation-detail-section="overview" id="reservation-overview"></section>
                <section class="asset-detail-section" data-reservation-detail-section="documents">
                  <div id="reservation-document-list" class="document-list"></div>
                  <form id="reservation-document-upload-form" class="document-form"><label>Bestand <input name="file" type="file" required /></label><label>Documentdatum <input name="documentDate" type="date" /></label><label>Omschrijving <input name="description" type="text" /></label><button type="submit">Uploaden en koppelen</button></form>
                  <form id="reservation-document-link-form" class="document-link-form"><label>Bestaand document <select name="documentId" required></select></label><button type="submit" class="secondary-button">Bestaand document koppelen</button></form>
                  <p id="reservation-document-status" class="document-status" aria-live="polite"></p>
                </section>
                <section class="asset-detail-section" data-reservation-detail-section="timeline"><div id="reservation-timeline" class="domain-timeline"></div></section>
              </section>
              <section id="reservation-empty-state" class="asset-panel empty-state"><strong>Kies een reservering</strong><p>Selecteer een reservering of maak een nieuwe aan.</p></section>
            </div>
          </div>
          </section>
        </section>
      </section>

      <section class="app-tab-section" data-main-section="cases" aria-label="Contracten">
        <section id="cases-root" class="assets-shell cases-shell">
          <header class="assets-heading dossier-heading">
            <div><p class="eyebrow">Vermogen</p><h1>Contracten</h1><p>Onderhoud contractdetails, documenten, transacties en de gekoppelde bezitting.</p></div>
          </header>
          <form id="case-filter-form" class="asset-filter-form case-filter-form">
            <label>Type <input name="caseType" type="search" placeholder="contract" /></label>
            <label>Status <select name="status"><option value="">Alle statussen</option>${caseStatusOptions()}</select></label>
          </form>
          <div class="assets-layout">
            <aside class="asset-list-panel"><div id="case-list"><p>Open een contract vanaf Dashboard of Vermogen.</p></div></aside>
            <div class="asset-content">
              <section id="case-form-panel" class="asset-panel hidden">
                <div class="panel-heading"><h2 id="case-form-title">Contract wijzigen</h2></div>
                <form id="case-form" class="asset-form case-form">
                  <input name="id" type="hidden" />
                  <label>Titel <input name="title" type="text" required /></label>
                  <label>Type <input name="caseType" type="text" list="case-type-examples" required /></label>
                  <datalist id="case-type-examples"><option value="contract"></option><option value="tax_return"></option><option value="home_purchase"></option><option value="renovation"></option><option value="estate"></option><option value="insurance_claim"></option><option value="emigration"></option><option value="other"></option></datalist>
                  <label>Startdatum <input name="startDate" type="date" /></label>
                  <label>Einddatum <input name="endDate" type="date" /></label>
                  <label>Looptijd in maanden <input name="termMonths" type="number" min="1" step="1" inputmode="numeric" /></label>
                  <label>Status <select name="status" required>${caseStatusOptions('active')}</select></label>
                  <label class="asset-description-field">Omschrijving <textarea name="description" rows="4"></textarea></label>
                  <div class="form-actions asset-form-actions"><button type="submit">Opslaan</button><button type="button" class="secondary-button" data-case-action="cancel-form">Annuleren</button></div>
                </form>
              </section>
              <section id="case-detail" class="asset-panel hidden">
                <div class="asset-detail-heading"><div><small>Contract</small><h2 id="case-detail-title"></h2></div><div class="row-actions"><button type="button" class="secondary-button" data-case-action="edit">Wijzigen</button><button type="button" class="secondary-button" data-case-action="delete">Verwijderen</button></div></div>
                <div id="case-category-summary" class="asset-relation-summary case-category-summary hidden" aria-live="polite"></div>
                <nav class="asset-detail-tabs" aria-label="Contract onderdelen">
                  <button type="button" class="asset-detail-tab active" data-case-detail-target="overview">Overzicht</button>
                  <button type="button" class="asset-detail-tab" data-case-detail-target="documents">Documenten</button>
                  <button type="button" class="asset-detail-tab" data-case-detail-target="transactions">Transacties</button>
                  <button type="button" class="asset-detail-tab" data-case-detail-target="relations">Gerelateerde objecten</button>
                  <button type="button" class="asset-detail-tab" data-case-detail-target="timeline">Tijdlijn</button>
                </nav>
                <section class="asset-detail-section active" data-case-detail-section="overview" id="case-overview"></section>
                <section class="asset-detail-section" data-case-detail-section="documents">
                  <div id="case-document-list" class="document-list"></div>
                  <form id="case-document-upload-form" class="document-form"><label>Bestand <input name="file" type="file" required /></label><label>Documentdatum <input name="documentDate" type="date" /></label><label>Omschrijving <input name="description" type="text" /></label><button type="submit">Uploaden en koppelen</button></form>
                  <form id="case-document-link-form" class="document-link-form"><label>Bestaand document <select name="documentId" required></select></label><button type="submit" class="secondary-button">Bestaand document koppelen</button></form>
                  <p id="case-document-status" class="document-status" aria-live="polite"></p>
                </section>
                <section class="asset-detail-section" data-case-detail-section="transactions">
                  <form id="case-transaction-link-form" class="asset-transaction-link-form"><label>Zoek transactie <input name="search" type="search" placeholder="Omschrijving of tegenpartij" /></label><button type="button" class="secondary-button" data-case-action="search-transactions">Zoeken</button><label>Transactie <select name="transactionId" required></select></label><button type="submit">Transactie koppelen</button></form>
                  <div id="case-transaction-list" class="asset-related-list"></div>
                </section>
                <section class="asset-detail-section" data-case-detail-section="relations">
                  <form id="case-relation-form" class="asset-relation-form">
                    <label>Objecttype <select name="targetType"><option value="asset">Bezitting</option><option value="liability">Schuld</option></select></label>
                    <label>Relatietype <select name="relationType"><option value="relates_to">Heeft betrekking op</option></select></label>
                    <label class="asset-relation-target">Gerelateerd object <select name="targetId" required></select></label>
                    <label>Geldig vanaf <input name="validFrom" type="date" /></label><label>Geldig tot <input name="validTo" type="date" /></label>
                    <label class="asset-relation-notes">Notitie <input name="notes" type="text" /></label>
                    <button type="submit">Relatie toevoegen</button>
                  </form>
                  <div id="case-relations" class="asset-related-list"></div>
                </section>
                 <section class="asset-detail-section" data-case-detail-section="timeline"><div id="case-timeline" class="domain-timeline"></div></section>
              </section>
                <section id="case-empty-state" class="asset-panel empty-state"><strong>Kies een contract</strong><p>Open een contract vanaf Dashboard of bij een bezitting onder Vermogen.</p></section>
            </div>
          </div>
        </section>
      </section>

      <section class="app-tab-section" data-main-section="configuration" aria-label="Configuratie">
        <section class="config-panel">
          <div class="config-heading">
            <small>Configuratie</small>
            <strong>Beheer instellingen</strong>
          </div>
          <div class="config-content">
          <nav class="config-menu" aria-label="Configuratie onderdelen" role="tablist">
            <button type="button" id="config-tab-system" class="config-tab active" data-config-target="system" role="tab" aria-controls="config-section-system" aria-selected="true">Systeemconfiguratie</button>
            <button type="button" id="config-tab-backup" class="config-tab" data-config-target="backup" role="tab" aria-controls="config-section-backup" aria-selected="false">Systeembackup</button>
            <button type="button" id="config-tab-categories" class="config-tab" data-config-target="categories" role="tab" aria-controls="config-section-categories" aria-selected="false">Onderhoud categorieen</button>
            <button type="button" id="config-tab-accounts" class="config-tab" data-config-target="accounts" role="tab" aria-controls="config-section-accounts" aria-selected="false">Onderhoud bankrekeningen</button>
            <button type="button" id="config-tab-hidden-transactions" class="config-tab" data-config-target="hidden-transactions" role="tab" aria-controls="config-section-hidden-transactions" aria-selected="false">Verborgen transacties</button>
          </nav>
          <section id="config-section-system" class="config-section system-config-section active" data-config-section="system" role="tabpanel" aria-labelledby="config-tab-system">
            <div id="system-configuration" class="system-configuration">
              <p class="system-config-status" role="status" aria-live="polite">Open Configuratie om de systeeminstellingen te laden.</p>
            </div>
          </section>
          <section id="config-section-backup" class="config-section backup-config-section" data-config-section="backup" role="tabpanel" aria-labelledby="config-tab-backup">
            <div id="backup-configuration" class="backup-configuration"><p class="system-config-status" role="status" aria-live="polite">Open Systeembackup om de instellingen te laden.</p></div>
          </section>
          <section id="config-section-categories" class="config-section" data-config-section="categories" role="tabpanel" aria-labelledby="config-tab-categories">
            <div>
              <p class="eyebrow">Categoriebeheer</p>
              <h2>Hoofdcategorieen</h2>
              <p>Deze lijst vormt straks de basis voor subcategorieen en automatische bankimportregels.</p>
            </div>
            <form id="main-category-form" class="inline-form">
              <label>
                Nieuwe hoofdcategorie
                <input name="name" type="text" placeholder="Bijvoorbeeld Huisdieren" required />
              </label>
              <button type="submit">Toevoegen</button>
            </form>
            <ol id="main-category-list" class="main-category-list">
              ${renderMainCategoryList()}
            </ol>
            <div id="subcategory-editor" class="subcategory-editor empty-state">
              <strong>Kies een hoofdcategorie</strong>
              <p>Klik op wijzigen naast een hoofdcategorie om subcategorieen te onderhouden.</p>
            </div>
          </section>
          <section id="config-section-accounts" class="config-section" data-config-section="accounts" role="tabpanel" aria-labelledby="config-tab-accounts">
            <div>
              <p class="eyebrow">Rekeningbeheer</p>
              <h2>Bankrekeningen</h2>
              <p>Deze rekeningen koppel je later aan bankimports en transacties.</p>
            </div>
            <form id="account-form" class="account-form">
              <input name="id" type="hidden" />
              <label>
                Naam
                <input name="name" type="text" placeholder="Triodos betaalrekening" required />
              </label>
              <label>
                IBAN
                <input name="iban" type="text" placeholder="NL00BANK0123456789" />
              </label>
              <label>
                Type
                <select name="type">
                  <option value="checking">Betaalrekening</option>
                  <option value="savings">Spaarrekening</option>
                </select>
              </label>
              <label>
                Bankprofiel
                <select name="bankProfile">
                  ${bankProfiles.map((profile) => `<option value="${profile}">${profile}</option>`).join('')}
                </select>
              </label>
              <label>
                Van wie
                <input name="owner" type="text" placeholder="Huishouden, Jan, Petra" />
              </label>
              <div class="form-actions">
                <button type="submit" id="account-submit">Rekening toevoegen</button>
                <button type="button" class="secondary-button hidden" id="account-cancel" data-action="cancel-account-edit">Annuleren</button>
              </div>
            </form>
            <ul id="account-list" class="account-list">${renderAccountList()}</ul>
          </section>
          <section id="config-section-hidden-transactions" class="config-section" data-config-section="hidden-transactions" role="tabpanel" aria-labelledby="config-tab-hidden-transactions">
            <div>
              <p class="eyebrow">Transacties</p>
              <h2>Verborgen transacties</h2>
              <p>Deze transacties doen niet mee in normale overzichten en grafieken, maar worden niet opnieuw geimporteerd.</p>
            </div>
            <div class="transaction-count"><span>Verborgen: <strong id="hidden-transaction-count">0</strong></span><span>Totaal: <strong id="hidden-transaction-total">${formatAmount(0)}</strong></span></div>
            <div id="hidden-transactions-result" class="table-wrap"><p>Verborgen transacties laden...</p></div>
          </section>
          </div>
        </section>
      </section>
      <dialog id="transaction-documents-dialog" class="document-dialog">
        <div class="document-dialog-heading">
          <div><small>Bijlagen</small><h2>Documenten bij transactie</h2></div>
          <form method="dialog"><button type="submit" class="icon-button" aria-label="Sluiten">&#10005;</button></form>
        </div>
        <div id="transaction-document-list" class="document-list"><p>Documenten laden...</p></div>
        <form id="document-upload-form" class="document-form">
          <label>Bestand <input name="file" type="file" required /></label>
          <label>Documentdatum <input name="documentDate" type="date" /></label>
          <label>Omschrijving <input name="description" type="text" /></label>
          <button type="submit">Uploaden en koppelen</button>
        </form>
        <form id="document-link-form" class="document-link-form">
          <label>Bestaand document <select name="documentId" required></select></label>
          <button type="submit" class="secondary-button">Bestaand document koppelen</button>
        </form>
        <p id="document-dialog-status" class="document-status" aria-live="polite"></p>
      </dialog>
    </section>
  `;

  document.querySelector('#main-category-form').addEventListener('submit', addMainCategory);
  document.querySelector('#main-category-list').addEventListener('click', handleMainCategoryAction);
  document.querySelector('#subcategory-editor').addEventListener('click', handleSubcategoryAction);
  document.querySelector('#account-form').addEventListener('submit', saveAccount);
  document.querySelector('#account-list').addEventListener('click', handleAccountAction);
  document.querySelector('#account-form').addEventListener('click', handleAccountAction);
  document.querySelector('#hidden-transactions-result').addEventListener('click', unhideFromTable);
  document.querySelector('#hidden-transactions-result').addEventListener('click', openDocumentsFromTable);
  document.querySelector('#chart-filter-form').addEventListener('change', handleChartFilterChange);
  document.querySelector('#chart-filter-form').addEventListener('click', handleDashboardPeriodClick);
  document.querySelector('#transaction-filter-form').addEventListener('change', handleTransactionFilterChange);
  document.querySelector('#transaction-filter-form').addEventListener('click', handleTransactionPeriodClick);
  document.querySelector('#account-transaction-search').addEventListener('input', resetAndLoadDashboardTransactions);
  document.querySelector('#transaction-result').addEventListener('click', categorizeFromTable);
  document.querySelector('#transaction-result').addEventListener('click', hideFromTable);
  document.querySelector('#transaction-result').addEventListener('click', openDocumentsFromTable);
  document.querySelector('#dashboard-transaction-result').addEventListener('click', categorizeFromTable);
  document.querySelector('#dashboard-transaction-result').addEventListener('click', hideFromTable);
  document.querySelector('#dashboard-transaction-result').addEventListener('click', openDocumentsFromTable);
  document.querySelector('#chart-result').addEventListener('click', handleDashboardChartSelection);
  document.querySelector('#dashboard-wealth-summary').addEventListener('click', handleDashboardWealthNavigation);
  document.querySelector('#dashboard-contracts').addEventListener('click', handleDashboardContractNavigation);
  document.querySelector('#transaction-result').addEventListener('click', handleAccountPagination);
  document.querySelector('#category-transaction-filter-form select[name="mainCategoryId"]').addEventListener('change', updateCategoryTransactionSubcategories);
  document.querySelector('#category-transaction-filter-form select[name="categoryId"]').addEventListener('change', resetAndLoadDashboardTransactions);
  document.querySelector('#manual-transaction-form').addEventListener('submit', saveManualTransaction);
  document.querySelector('#manual-transaction-form select[name="direction"]').addEventListener('change', updateManualTransactionCategories);
  document.querySelectorAll('.main-tab').forEach((button) => button.addEventListener('click', switchMainSection));
  document.querySelectorAll('.wealth-tab').forEach((button) => button.addEventListener('click', switchWealthSection));
  document.querySelectorAll('.config-tab').forEach((button) => button.addEventListener('click', switchConfigSection));
  document.querySelector('#system-configuration').addEventListener('click', handleSystemConfigurationClick);
  document.querySelector('#system-configuration').addEventListener('change', updateSystemConfigurationFields);
  document.querySelector('#system-configuration').addEventListener('submit', saveSystemConfiguration);
  document.querySelector('#backup-configuration').addEventListener('click', handleBackupConfigurationClick);
  document.querySelector('#backup-configuration').addEventListener('input', invalidateBackupConfiguration);
  document.querySelector('#backup-configuration').addEventListener('submit', saveBackupConfiguration);
  document.querySelector('#import-form').addEventListener('submit', previewImport);
  document.querySelector('#document-upload-form').addEventListener('submit', uploadTransactionDocument);
  document.querySelector('#document-link-form').addEventListener('submit', linkExistingDocument);
  document.querySelector('#transaction-document-list').addEventListener('click', unlinkTransactionDocument);
  document.querySelector('#assets-root').addEventListener('click', handleAssetAction);
  document.querySelector('#asset-filter-form').addEventListener('change', loadAssets);
  document.querySelector('#asset-form').addEventListener('submit', saveAsset);
  document.querySelector('#asset-valuation-form').addEventListener('submit', saveAssetValuation);
  document.querySelector('#asset-contract-form').addEventListener('submit', saveAssetContract);
  document.querySelector('#asset-document-upload-form').addEventListener('submit', uploadAssetDocument);
  document.querySelector('#asset-document-link-form').addEventListener('submit', linkExistingAssetDocument);
  document.querySelector('#asset-transaction-link-form').addEventListener('submit', linkAssetTransaction);
  document.querySelector('#liabilities-root').addEventListener('click', handleLiabilityAction);
  document.querySelector('#liability-filter-form').addEventListener('change', loadLiabilities);
  document.querySelector('#liability-form').addEventListener('submit', saveLiability);
  document.querySelector('#liability-contract-form').addEventListener('submit', saveLiabilityContract);
  document.querySelector('#liability-document-upload-form').addEventListener('submit', uploadLiabilityDocument);
  document.querySelector('#liability-document-link-form').addEventListener('submit', linkExistingLiabilityDocument);
  document.querySelector('#liability-transaction-link-form').addEventListener('submit', linkLiabilityTransaction);
  document.querySelector('#reservations-root').addEventListener('click', handleReservationAction);
  document.querySelector('#reservation-filter-form').addEventListener('change', loadReservations);
  document.querySelector('#reservation-form').addEventListener('submit', saveReservation);
  document.querySelector('#reservation-document-upload-form').addEventListener('submit', uploadReservationDocument);
  document.querySelector('#reservation-document-link-form').addEventListener('submit', linkExistingReservationDocument);
  document.querySelector('#cases-root').addEventListener('click', handleCaseAction);
  document.querySelector('#case-filter-form input[name="caseType"]').addEventListener('input', loadCases);
  document.querySelector('#case-filter-form').addEventListener('change', loadCases);
  document.querySelector('#case-form').addEventListener('submit', saveCase);
  document.querySelector('#case-document-upload-form').addEventListener('submit', uploadCaseDocument);
  document.querySelector('#case-document-link-form').addEventListener('submit', linkExistingCaseDocument);
  document.querySelector('#case-transaction-link-form').addEventListener('submit', linkCaseTransaction);
  document.querySelector('#case-relation-form').addEventListener('submit', saveCaseRelation);
  document.querySelector('#case-relation-form select[name="targetType"]').addEventListener('change', updateCaseRelationForm);
  app.addEventListener('click', openRelatedDomainDetail);
  loadDashboardTransactions();
  loadCategoryChart();
  loadDashboardWealth();
  loadDashboardContracts();
  loadHiddenTransactions();
}

function handleChartFilterChange(event) {
  loadCategoryChart();
  if (event.target.name === 'mainCategoryId' && event.target.value) {
    const mainCategory = mainCategoriesState.find((item) => item.id === Number(event.target.value));
    if (mainCategory) {
      selectDashboardTransactions({ type: 'mainCategory', id: mainCategory.id, label: mainCategory.name });
    }
  }
}

function handleDashboardPeriodClick(event) {
  const yearButton = event.target.closest('button[data-dashboard-year]');
  const monthButton = event.target.closest('button[data-dashboard-month]');
  if (!yearButton && !monthButton) {
    return;
  }

  const form = event.currentTarget;
  if (yearButton) {
    dashboardFilterYear = yearButton.dataset.dashboardYear;
    dashboardFilterMonth = '';
  } else if (dashboardFilterYear) {
    dashboardFilterMonth = dashboardFilterMonth === monthButton.dataset.dashboardMonth ? '' : monthButton.dataset.dashboardMonth;
  }
  form.elements.year.value = dashboardFilterYear;
  form.elements.month.value = dashboardFilterMonth;
  updateDashboardPeriodButtons();
  loadCategoryChart();
  reloadSelectedDashboardTransactions();
}

function updateDashboardPeriodButtons() {
  document.querySelectorAll('button[data-dashboard-year]').forEach((button) => {
    const active = button.dataset.dashboardYear === dashboardFilterYear;
    button.classList.toggle('active', active);
    button.setAttribute('aria-pressed', String(active));
  });
  document.querySelectorAll('button[data-dashboard-month]').forEach((button) => {
    const active = button.dataset.dashboardMonth === dashboardFilterMonth;
    button.classList.toggle('active', active);
    button.setAttribute('aria-pressed', String(active));
    button.disabled = !dashboardFilterYear;
  });
}

async function loadCategoryChart() {
  const form = document.querySelector('#chart-filter-form');
  const result = document.querySelector('#chart-result');
  const params = new URLSearchParams();
  if (form.elements.mainCategoryId.value) {
    params.set('mainCategoryId', form.elements.mainCategoryId.value);
  }
  applyYearMonthFilter(params, form.elements.year.value, form.elements.month.value);

  result.innerHTML = 'Grafiek laden...';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/reports/main-category-period?${params}`);
    const totals = await response.json();
    if (!response.ok) {
      throw new Error(totals.error || 'Grafiek laden mislukt');
    }
    result.innerHTML = renderCategoryChart(totals);
  } catch (error) {
    result.innerHTML = `<p class="error">${error.message}</p>`;
  }
}

function renderCategoryChart(totals) {
  if (!totals.length) {
    return '<p>Nog geen gecategoriseerde bedragen voor deze periode.</p>';
  }
  const total = totals.reduce((sum, item) => sum + item.amountCents, 0);
  let offset = 0;
  const colors = ['#fca311', '#2f7d32', '#394867', '#9f2d20', '#7b5ea7', '#0f8b8d', '#d45d00', '#64748b'];
  const slices = totals.map((item, index) => {
    const label = categoryChartItemName(item);
    const selectionAttributes = dashboardChartSelectionAttributes(item, label);
    const share = total === 0 ? 0 : item.amountCents / total;
    const dash = `${share * 100} ${100 - share * 100}`;
    const slice = `
      <circle class="pie-slice ${dashboardChartSelectionActive(item) ? 'active' : ''}" ${selectionAttributes} r="16" cx="20" cy="20" fill="transparent" stroke="${colors[index % colors.length]}" stroke-width="32" stroke-dasharray="${dash}" stroke-dashoffset="${-offset}">
        <title>${escapeHtml(label)}: ${formatAmount(item.amountCents)}</title>
      </circle>
    `;
    offset += share * 100;
    return slice;
  }).join('');
  const legend = totals.map((item, index) => {
    const label = categoryChartItemName(item);
    return `<li><button type="button" class="pie-legend-button ${dashboardChartSelectionActive(item) ? 'active' : ''}" ${dashboardChartSelectionAttributes(item, label)}><span><i style="background:${colors[index % colors.length]}"></i>${escapeHtml(label)}</span><strong>${formatAmount(item.amountCents)}</strong></button></li>`;
  }).join('');
  const breakdown = totals.some((item) => item.categoryId) ? 'subcategorie(en)' : 'hoofdcategorie(en)';

  return `
    <strong>${formatAmount(total)}</strong>
    <span>${chartTotalLabel()} verdeeld over ${totals.length} ${breakdown}</span>
    <div class="pie-chart-layout">
      <svg class="pie-chart" viewBox="0 0 40 40" role="img" aria-label="${chartTotalLabel()} per ${breakdown}">${slices}</svg>
      <ul class="pie-legend">${legend}</ul>
    </div>
  `;
}

function categoryChartItemName(item) {
  return item.categoryName || item.mainCategoryName;
}

function dashboardChartSelectionAttributes(item, label) {
  return `data-dashboard-main-category="${item.mainCategoryId}" data-dashboard-category="${item.categoryId || ''}" data-dashboard-label="${escapeHtml(label)}"`;
}

function dashboardChartSelectionActive(item) {
  if (!dashboardTransactionSelection) {
    return false;
  }
  if (item.categoryId) {
    return dashboardTransactionSelection.type === 'category' && dashboardTransactionSelection.id === item.categoryId;
  }
  return dashboardTransactionSelection.type === 'mainCategory' && dashboardTransactionSelection.id === item.mainCategoryId;
}

function chartTotalLabel() {
  const form = document.querySelector('#chart-filter-form');
  const mainCategory = mainCategoriesState.find((category) => String(category.id) === form?.elements.mainCategoryId.value);
  return mainCategory?.name === 'Inkomsten' ? 'Inkomsten' : 'Uitgaven';
}

function renderDashboardAccountOptions() {
  if (!accountsState.length) {
    return '<option value="">Geen rekeningen</option>';
  }
  return accountsState.map((account) => `<option value="${account.id}">${escapeHtml(account.name)}</option>`).join('');
}

function renderTransactionAccountOptions() {
  return '<option value="">Alle rekeningen</option>' + renderDashboardAccountOptions();
}

function renderManualAccountOptions() {
  if (!accountsState.length) return '<option value="">Maak eerst een rekening aan</option>';
  return accountsState.map((account) => `<option value="${account.id}">${escapeHtml(account.name)}</option>`).join('');
}

function manualTransactionCategoryOptions(direction) {
  const kind = direction === 'income' ? 'income' : 'expense';
  const options = categoriesState.filter((category) => category.kind === kind).sort((left, right) => left.name.localeCompare(right.name, 'nl'));
  return '<option value="">Ongecategoriseerd</option>' + options.map((category) => `<option value="${category.id}">${escapeHtml(category.name)}</option>`).join('');
}

function updateManualTransactionCategories(event) {
  const form = event.currentTarget.form;
  form.elements.categoryId.innerHTML = manualTransactionCategoryOptions(event.currentTarget.value);
}

async function saveManualTransaction(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const status = document.querySelector('#manual-transaction-status');
  const submit = form.querySelector('button[type="submit"]');
  const cents = amountInputToCents(form.elements.amount.value);
  const input = {
    accountId: Number(form.elements.accountId.value),
    categoryId: Number(form.elements.categoryId.value),
    bookedOn: form.elements.bookedOn.value,
    description: form.elements.description.value,
    counterparty: form.elements.counterparty.value,
    amountCents: form.elements.direction.value === 'expense' ? -Math.abs(cents) : Math.abs(cents),
  };
  submit.disabled = true;
  status.textContent = 'Geldstroom opslaan...';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/transactions`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || 'Geldstroom opslaan mislukt');
    const accountID = input.accountId;
    form.reset();
    form.elements.bookedOn.value = localDateValue();
    form.elements.accountId.value = String(accountID);
    form.elements.categoryId.innerHTML = manualTransactionCategoryOptions('expense');
    status.textContent = 'Geldstroom toegevoegd.';
    accountsState = await loadAccounts();
    refreshAccounts();
    form.elements.accountId.value = String(accountID);
    await refreshTransactionYears();
    accountTransactionPage.offset = 0;
    loadDashboardTransactions();
    loadDashboardWealth();
  } catch (error) {
    status.textContent = error.message;
  } finally {
    submit.disabled = false;
  }
}

function defaultTransactionYear() {
  const currentYear = new Date().getFullYear();
  return String(transactionYearsState.includes(currentYear) ? currentYear : (transactionYearsState[0] || ''));
}

function transactionYearButtons() {
  if (!transactionYearsState.length) {
    return '<span>Geen jaren beschikbaar</span>';
  }
  return transactionYearsState.map((year) => `
    <button type="button" class="period-button ${String(year) === transactionFilterYear ? 'active' : ''}" data-filter-year="${year}" aria-pressed="${String(year) === transactionFilterYear}">${year}</button>
  `).join('');
}

function transactionMonthButtons() {
  const months = ['Jan', 'Feb', 'Maa', 'Apr', 'Mei', 'Jun', 'Jul', 'Aug', 'Sep', 'Okt', 'Nov', 'Dec'];
  return months.map((month, index) => `
    <button type="button" class="period-button ${String(index + 1) === transactionFilterMonth ? 'active' : ''}" data-filter-month="${index + 1}" aria-pressed="${String(index + 1) === transactionFilterMonth}" ${transactionFilterYear ? '' : 'disabled'}>${month}</button>
  `).join('');
}

function dashboardYearButtons() {
  if (!transactionYearsState.length) {
    return '<span>Geen jaren beschikbaar</span>';
  }
  return transactionYearsState.map((year) => `
    <button type="button" class="period-button ${String(year) === dashboardFilterYear ? 'active' : ''}" data-dashboard-year="${year}" aria-pressed="${String(year) === dashboardFilterYear}">${year}</button>
  `).join('');
}

function dashboardMonthButtons() {
  const months = ['Jan', 'Feb', 'Maa', 'Apr', 'Mei', 'Jun', 'Jul', 'Aug', 'Sep', 'Okt', 'Nov', 'Dec'];
  return months.map((month, index) => `
    <button type="button" class="period-button ${String(index + 1) === dashboardFilterMonth ? 'active' : ''}" data-dashboard-month="${index + 1}" aria-pressed="${String(index + 1) === dashboardFilterMonth}" ${dashboardFilterYear ? '' : 'disabled'}>${month}</button>
  `).join('');
}

async function refreshTransactionYears() {
  transactionYearsState = await loadTransactionYears();
  const form = document.querySelector('#transaction-filter-form');
  if (!form) {
    return;
  }
  if (!transactionYearsState.includes(Number(transactionFilterYear))) {
    transactionFilterYear = defaultTransactionYear();
    transactionFilterMonth = '';
  }
  form.elements.year.value = transactionFilterYear;
  form.elements.month.value = transactionFilterMonth;
  document.querySelector('#transaction-year-buttons').innerHTML = transactionYearButtons();
  document.querySelector('#transaction-month-buttons').innerHTML = transactionMonthButtons();
  if (!transactionYearsState.includes(Number(dashboardFilterYear))) {
    dashboardFilterYear = defaultTransactionYear();
    dashboardFilterMonth = '';
  }
  const chartForm = document.querySelector('#chart-filter-form');
  chartForm.elements.year.value = dashboardFilterYear;
  chartForm.elements.month.value = dashboardFilterMonth;
  document.querySelector('#dashboard-year-buttons').innerHTML = dashboardYearButtons();
  document.querySelector('#dashboard-month-buttons').innerHTML = dashboardMonthButtons();
}

function mainCategoryOptionsWithAll() {
  return '<option value="">Alle hoofdcategorieen</option><option value="uncategorized">Ongecategoriseerd</option>' + mainCategoryOptions();
}

function chartMainCategoryOptions() {
  return '<option value="">Alle hoofdcategorieen</option>' + mainCategoryOptions();
}

function renderAccountList() {
  if (!accountsState.length) {
    return '<li><span>Nog geen bankrekeningen</span></li>';
  }
  return accountsState.map((account) => `
    <li>
      <span>
        <strong>${escapeHtml(account.name)}</strong>
        <small>${account.type === 'savings' ? 'Spaarrekening' : 'Betaalrekening'}${account.owner ? ` / ${escapeHtml(account.owner)}` : ''}</small>
        <small>Importprofiel: ${escapeHtml(account.bankProfile || 'generic')}</small>
        <small>${escapeHtml(account.iban || 'Geen IBAN')}</small>
      </span>
      <span class="row-actions">
        <button type="button" class="icon-button" data-action="edit-account" data-id="${account.id}" aria-label="Wijzig ${escapeHtml(account.name)}">&#9998;</button>
        <button type="button" class="icon-button danger" data-action="delete-account" data-id="${account.id}" aria-label="Verwijder ${escapeHtml(account.name)}">&#10005;</button>
      </span>
    </li>
  `).join('');
}

function renderImportAccountOptions() {
  if (!accountsState.length) {
    return '<option value="">Geen bankrekeningen ingevoerd</option>';
  }
  return accountsState.map((account) => `
    <option value="${account.id}">${escapeHtml(account.name)} (${escapeHtml(account.bankProfile || 'generic')})</option>
  `).join('');
}

async function loadDashboardTransactions(event, uncategorizedOnly = false) {
  event?.preventDefault();
  accountTransactionsUncategorizedOnly = uncategorizedOnly;
  const result = document.querySelector('#transaction-result');
  const accountID = document.querySelector('#dashboard-account-select')?.value;
  const form = document.querySelector('#transaction-filter-form');
  const categoryForm = document.querySelector('#category-transaction-filter-form');
  accountTransactionPage.limit = Number(form.elements.limit.value);

  const params = new URLSearchParams();
  if (accountID) {
    params.set('accountId', accountID);
  }
  const mainCategoryValue = categoryForm.elements.mainCategoryId.value;
  if (mainCategoryValue === 'uncategorized') {
    params.set('uncategorized', 'true');
  } else if (categoryForm.elements.categoryId.value) {
    params.set('categoryId', categoryForm.elements.categoryId.value);
  } else if (mainCategoryValue) {
    params.set('mainCategoryId', mainCategoryValue);
  }
  applyYearMonthFilter(params, form.elements.year.value, form.elements.month.value);
  const search = document.querySelector('#account-transaction-search').value.trim();
  if (search) {
    params.set('search', search);
  }
  if (uncategorizedOnly) {
    params.set('uncategorized', 'true');
  }
  const summaryParams = new URLSearchParams(params);
  params.set('limit', String(accountTransactionPage.limit));
  params.set('offset', String(accountTransactionPage.offset));

  result.innerHTML = '<p>Transacties laden...</p>';
  try {
    const [response, summaryResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/transactions?${params}`),
      fetch(`${apiBaseUrl}/api/v1/transactions/summary?${summaryParams}`),
    ]);
    const [transactions, summary] = await Promise.all([response.json(), summaryResponse.json()]);
    if (!response.ok) {
      throw new Error(transactions.error || 'Transacties laden mislukt');
    }
    if (!summaryResponse.ok) {
      throw new Error(summary.error || 'Transactiesamenvatting laden mislukt');
    }
    updateShownTransactionSummary(summary);
    result.innerHTML = renderTransactionTable(transactions) + renderPagination('account', summary.count, accountTransactionPage);
  } catch (error) {
    updateShownTransactionSummary({ count: 0, amountCents: 0 });
    result.innerHTML = `<p class="error">${error.message}</p>`;
  }
}

function updateShownTransactionSummary(summary) {
  const count = document.querySelector('#shown-transaction-count');
  const total = document.querySelector('#shown-transaction-total');
  if (!count || !total) {
    return;
  }
  count.textContent = String(summary.count || 0);
  total.textContent = formatAmount(summary.amountCents || 0);
}

function resetAndLoadDashboardTransactions(event) {
  accountTransactionPage.offset = 0;
  loadDashboardTransactions(event, false);
}

function handleTransactionFilterChange(event) {
  resetAndLoadDashboardTransactions(event);
}

function handleTransactionPeriodClick(event) {
  const yearButton = event.target.closest('button[data-filter-year]');
  const monthButton = event.target.closest('button[data-filter-month]');
  if (!yearButton && !monthButton) {
    return;
  }

  const form = event.currentTarget;
  if (yearButton) {
    transactionFilterYear = yearButton.dataset.filterYear;
    transactionFilterMonth = '';
  } else if (transactionFilterYear) {
    transactionFilterMonth = transactionFilterMonth === monthButton.dataset.filterMonth ? '' : monthButton.dataset.filterMonth;
  }
  form.elements.year.value = transactionFilterYear;
  form.elements.month.value = transactionFilterMonth;
  updateTransactionPeriodButtons();
  resetAndLoadDashboardTransactions(event);
}

function updateTransactionPeriodButtons() {
  document.querySelectorAll('button[data-filter-year]').forEach((button) => {
    const active = button.dataset.filterYear === transactionFilterYear;
    button.classList.toggle('active', active);
    button.setAttribute('aria-pressed', String(active));
  });
  document.querySelectorAll('button[data-filter-month]').forEach((button) => {
    const active = button.dataset.filterMonth === transactionFilterMonth;
    button.classList.toggle('active', active);
    button.setAttribute('aria-pressed', String(active));
    button.disabled = !transactionFilterYear;
  });
}

function applyYearMonthFilter(params, yearValue, monthValue) {
  if (!yearValue) {
    return;
  }
  const year = Number(yearValue);
  if (!monthValue) {
    params.set('from', `${year}-01-01`);
    params.set('to', `${year}-12-31`);
    return;
  }
  const month = Number(monthValue);
  const paddedMonth = String(month).padStart(2, '0');
  const lastDay = new Date(Date.UTC(year, month, 0)).getUTCDate();
  params.set('from', `${year}-${paddedMonth}-01`);
  params.set('to', `${year}-${paddedMonth}-${String(lastDay).padStart(2, '0')}`);
}

function loadUncategorizedTransactions() {
  accountTransactionPage.offset = 0;
  loadDashboardTransactions(undefined, true);
}

async function loadCategoryTransactions(event) {
  accountTransactionPage.offset = categoryTransactionPage.offset;
  return loadDashboardTransactions(event, false);
}

function resetAndLoadCategoryTransactions(event) {
  categoryTransactionPage.offset = 0;
  loadCategoryTransactions(event);
}

function updateCategoryTransactionSubcategories() {
  const form = document.querySelector('#category-transaction-filter-form');
  if (form.elements.mainCategoryId.value === 'uncategorized') {
    form.elements.categoryId.innerHTML = '<option value="">Niet van toepassing</option>';
    form.elements.categoryId.disabled = true;
  } else if (!form.elements.mainCategoryId.value) {
    form.elements.categoryId.innerHTML = '<option value="">Kies eerst een hoofdcategorie</option>';
    form.elements.categoryId.disabled = true;
  } else {
    form.elements.categoryId.innerHTML = subcategoryOptions(form.elements.mainCategoryId.value);
    form.elements.categoryId.disabled = false;
  }
  accountTransactionPage.offset = 0;
  loadDashboardTransactions();
}

function handleDashboardChartSelection(event) {
  const target = event.target.closest('[data-dashboard-main-category]');
  if (!target) {
    return;
  }
  const categoryID = Number(target.dataset.dashboardCategory);
  selectDashboardTransactions({
    type: categoryID ? 'category' : 'mainCategory',
    id: categoryID || Number(target.dataset.dashboardMainCategory),
    label: target.dataset.dashboardLabel,
  });
}

function selectDashboardTransactions(selection) {
  dashboardTransactionSelection = selection;
  updateDashboardSelectionStyles();
  loadSelectedDashboardTransactions();
}

function updateDashboardSelectionStyles() {
  document.querySelectorAll('[data-dashboard-main-category]').forEach((element) => {
    const categoryID = Number(element.dataset.dashboardCategory);
    const active = categoryID
      ? dashboardTransactionSelection?.type === 'category' && dashboardTransactionSelection.id === categoryID
      : dashboardTransactionSelection?.type === 'mainCategory' && dashboardTransactionSelection.id === Number(element.dataset.dashboardMainCategory);
    element.classList.toggle('active', active);
  });
}

async function loadSelectedDashboardTransactions() {
  if (!dashboardTransactionSelection) {
    return;
  }
  const label = document.querySelector('#dashboard-transaction-label');
  const result = document.querySelector('#dashboard-transaction-result');
  const chartForm = document.querySelector('#chart-filter-form');
  const params = new URLSearchParams({ limit: '100' });
  if (dashboardTransactionSelection.type === 'account') {
    params.set('accountId', String(dashboardTransactionSelection.id));
  } else if (dashboardTransactionSelection.type === 'category') {
    params.set('categoryId', String(dashboardTransactionSelection.id));
  } else {
    params.set('mainCategoryId', String(dashboardTransactionSelection.id));
  }
  applyYearMonthFilter(params, chartForm.elements.year.value, chartForm.elements.month.value);

  label.textContent = `${dashboardTransactionSelection.label} · ${dashboardPeriodLabel()}`;
  result.innerHTML = '<p>Transacties laden...</p>';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/transactions?${params}`);
    const transactions = await response.json();
    if (!response.ok) {
      throw new Error(transactions.error || 'Transacties laden mislukt');
    }
    result.innerHTML = renderTransactionTable(transactions);
  } catch (error) {
    result.innerHTML = `<p class="error">${error.message}</p>`;
  }
}

function reloadSelectedDashboardTransactions() {
  if (dashboardTransactionSelection) {
    loadSelectedDashboardTransactions();
  }
}

function dashboardPeriodLabel() {
  const monthNames = ['januari', 'februari', 'maart', 'april', 'mei', 'juni', 'juli', 'augustus', 'september', 'oktober', 'november', 'december'];
  if (dashboardFilterMonth) {
    return `${monthNames[Number(dashboardFilterMonth) - 1]} ${dashboardFilterYear}`;
  }
  return dashboardFilterYear || 'alle jaren';
}

function renderPagination(scope, totalCount, page) {
  const pageCount = Math.max(1, Math.ceil(totalCount / page.limit));
  const currentPage = Math.floor(page.offset / page.limit) + 1;
  if (pageCount <= 1) {
    return '<div class="pagination"><span>Pagina 1 van 1</span></div>';
  }
  const start = Math.max(1, currentPage - 3);
  const end = Math.min(pageCount, currentPage + 3);
  const links = [];
  for (let pageNumber = start; pageNumber <= end; pageNumber += 1) {
    links.push(`<button type="button" class="page-link ${pageNumber === currentPage ? 'active' : ''}" data-page-number="${pageNumber}" ${pageNumber === currentPage ? 'aria-current="page"' : ''}>${pageNumber}</button>`);
  }
  return `
    <div class="pagination" data-pagination="${scope}">
      <span>Pagina ${currentPage} van ${pageCount}</span>
      <span class="page-links">${links.join('')}</span>
    </div>
  `;
}

function handleAccountPagination(event) {
  const button = event.target.closest('button[data-page-number]');
  if (!button) {
    return;
  }
  accountTransactionPage.offset = (Number(button.dataset.pageNumber) - 1) * accountTransactionPage.limit;
  loadDashboardTransactions(undefined, accountTransactionsUncategorizedOnly);
}

function handleCategoryPagination(event) {
  const button = event.target.closest('button[data-page-number]');
  if (!button) {
    return;
  }
  categoryTransactionPage.offset = (Number(button.dataset.pageNumber) - 1) * categoryTransactionPage.limit;
  loadCategoryTransactions();
}

function renderTransactionTable(transactions) {
  if (!transactions.length) {
    return '<p>Geen transacties gevonden voor deze selectie.</p>';
  }
  const rows = transactions.map((transaction) => `
    <tr data-transaction-id="${transaction.id}" data-counterparty="${escapeHtml(transaction.counterparty || '')}" data-category-id="${transaction.categoryId || ''}">
      <td>${escapeHtml(transaction.bookedOn)}</td>
      <td><span class="transaction-description">${escapeHtml(transaction.description)}</span>${renderDocumentControl(transaction)}</td>
      <td>${escapeHtml(transaction.counterparty || '')}</td>
      <td class="categorize-cell">
        <span data-category-label>${escapeHtml(categoryName(transaction.categoryId))}</span>
        ${transaction.hidden ? renderUnhideControls() : renderCategorizeControls(transaction.categoryId)}
      </td>
      <td class="amount">${formatAmount(transaction.amountCents)}</td>
    </tr>
  `).join('');

  return `
    <table>
      <thead><tr><th>Datum</th><th>Omschrijving</th><th>Tegenpartij</th><th>Subcategorie</th><th>Bedrag</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderDocumentControl(transaction) {
  const count = Number(transaction.documentCount || 0);
  return `<button type="button" class="document-button" data-action="open-transaction-documents" aria-label="Documenten bij transactie">&#128206;<span data-document-count>${count > 0 ? count : ''}</span></button>`;
}

function renderCategorizeControls(categoryID) {
  return `
    <span class="categorize-controls">
      <select data-category-select>${transactionCategoryOptions(categoryID)}</select>
      <button type="button" class="icon-button" data-action="categorize" aria-label="Sla categorie op">&#128190;</button>
      <button type="button" class="icon-button" data-action="categorize-same-counterparty" aria-label="Categoriseer alle ongecategoriseerde transacties met deze tegenpartij">&#128101;</button>
      <button type="button" class="icon-button danger" data-action="hide-transaction" aria-label="Verberg transactie">&#128065;</button>
    </span>
  `;
}

function renderUnhideControls() {
  return `
    <span class="categorize-controls">
      <button type="button" class="icon-button" data-action="unhide-transaction" aria-label="Maak transactie zichtbaar">&#8617;</button>
    </span>
  `;
}

function categoryName(categoryID) {
  if (!categoryID) {
    return 'Ongecategoriseerd';
  }
  return categoriesState.find((category) => category.id === categoryID)?.name || 'Onbekend';
}

function transactionCategoryOptions(selectedID) {
  const options = ['<option value="">Kies subcategorie</option>'];
  const sortedCategories = [...categoriesState].sort((a, b) => a.name.localeCompare(b.name, 'nl', { sensitivity: 'base' }));
  for (const category of sortedCategories) {
    options.push(`<option value="${category.id}" ${category.id === selectedID ? 'selected' : ''}>${escapeHtml(category.name)}</option>`);
  }
  return options.join('');
}

async function categorizeFromTable(event) {
  const button = event.target.closest('button[data-action="categorize"], button[data-action="categorize-same-counterparty"]');
  if (!button) {
    return;
  }

  const row = button.closest('tr[data-transaction-id]');
  const categoryID = Number(row.querySelector('[data-category-select]').value);
  if (!categoryID) {
    window.alert('Kies eerst een subcategorie.');
    return;
  }

  const applyToSameCounterparty = button.dataset.action === 'categorize-same-counterparty';
  const response = await fetch(`${apiBaseUrl}/api/v1/transactions/${row.dataset.transactionId}/category`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ categoryId: categoryID, applyToSameCounterparty }),
  });
  const data = await response.json();
  if (!response.ok) {
    window.alert(data.error || 'Categoriseren mislukt');
    return;
  }

  updateVisibleCategorizedRows(row, categoryID, applyToSameCounterparty);
  showCategorizeStatus(row, `${data.updated} transactie(s) gecategoriseerd.`);
}

async function hideFromTable(event) {
  const button = event.target.closest('button[data-action="hide-transaction"]');
  if (!button) {
    return;
  }
  const row = button.closest('tr[data-transaction-id]');
  if (!window.confirm('Deze transactie verbergen uit gewone overzichten en grafieken?')) {
    return;
  }
  const response = await fetch(`${apiBaseUrl}/api/v1/transactions/${row.dataset.transactionId}/hidden`, { method: 'PATCH' });
  if (!response.ok) {
    const data = await response.json();
    window.alert(data.error || 'Transactie verbergen mislukt');
    return;
  }
  row.remove();
  incrementTransactionSummary(-1);
}

async function unhideFromTable(event) {
  const button = event.target.closest('button[data-action="unhide-transaction"]');
  if (!button) {
    return;
  }
  const row = button.closest('tr[data-transaction-id]');
  const response = await fetch(`${apiBaseUrl}/api/v1/transactions/${row.dataset.transactionId}/hidden`, { method: 'DELETE' });
  if (!response.ok) {
    const data = await response.json();
    window.alert(data.error || 'Transactie terugzetten mislukt');
    return;
  }
  row.remove();
  incrementTransactionSummary(1);
  loadHiddenTransactions();
}

async function openDocumentsFromTable(event) {
  const button = event.target.closest('button[data-action="open-transaction-documents"]');
  if (!button) {
    return;
  }
  const row = button.closest('tr[data-transaction-id]');
  const dialog = document.querySelector('#transaction-documents-dialog');
  dialog.dataset.transactionId = row.dataset.transactionId;
  document.querySelector('#document-dialog-status').textContent = '';
  dialog.showModal();
  await loadTransactionDocuments();
}

async function loadTransactionDocuments() {
  const dialog = document.querySelector('#transaction-documents-dialog');
  const transactionID = dialog.dataset.transactionId;
  const list = document.querySelector('#transaction-document-list');
  list.innerHTML = '<p>Documenten laden...</p>';
  try {
    const [linkedResponse, allResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/transactions/${transactionID}/documents`),
      fetch(`${apiBaseUrl}/api/v1/documents`),
    ]);
    const linked = await linkedResponse.json();
    const all = await allResponse.json();
    if (!linkedResponse.ok || !allResponse.ok) {
      throw new Error(linked.error || all.error || 'Documenten laden mislukt');
    }
    if (dialog.dataset.transactionId !== transactionID) {
      return;
    }
    list.innerHTML = renderTransactionDocuments(linked);
    renderExistingDocumentOptions(all, linked);
    updateDocumentCount(transactionID, linked.length);
  } catch (error) {
    if (dialog.dataset.transactionId === transactionID) {
      list.innerHTML = `<p class="error">${escapeHtml(error.message)}</p>`;
    }
  }
}

function renderTransactionDocuments(documents) {
  if (!documents.length) {
    return '<p>Nog geen documenten gekoppeld.</p>';
  }
  return `<ul>${documents.map((document) => `
    <li>
      <div><a href="${apiBaseUrl}/api/v1/documents/${document.id}/content" target="_blank" rel="noopener">${escapeHtml(document.originalFilename)}</a><small>${escapeHtml(document.documentDate || document.mimeType)} &middot; ${formatFileSize(document.sizeBytes)}</small>${document.description ? `<span>${escapeHtml(document.description)}</span>` : ''}${renderDocumentTranslation(document)}</div>
      <button type="button" class="icon-button danger" data-action="unlink-document" data-document-id="${document.id}" aria-label="Verwijder koppeling">&#10005;</button>
    </li>
  `).join('')}</ul>`;
}

function renderDocumentTranslation(document) {
  switch (document.translationStatus) {
  case 'pending': return '<small>Documentanalyse wordt gemaakt; OCR en AI kunnen meerdere minuten duren...</small>';
  case 'completed': return document.analysis || document.translationText || document.text
    ? `<details class="document-analysis"><summary>Nederlandse documentanalyse</summary><div class="document-analysis-body">${renderDocumentAnalysisResult(document)}</div></details>`
    : '<small>Geen documentanalyse beschikbaar.</small>';
  case 'unsupported': return '<small>Documentanalyse is alleen beschikbaar voor PDF-bestanden.</small>';
  case 'failed': return `<small class="error">Documentanalyse is mislukt.${document.text ? ` ${escapeHtml(document.text)}` : ''}</small>`;
  default: return '';
  }
}

function renderDocumentAnalysisResult(document) {
  const blocks = [];
  if (document.analysis) {
    blocks.push(renderStructuredDocumentAnalysis(document.analysis));
  } else if (document.translationText) {
    blocks.push(renderDocumentAnalysis(document.translationText));
  }
  if (document.text) {
    blocks.push(`<details class="document-full-text"><summary>Volledige documenttekst</summary><pre>${escapeHtml(document.text)}</pre></details>`);
  }
  return blocks.join('');
}

function renderStructuredDocumentAnalysis(value) {
  if (value == null) return '';
  if (typeof value === 'string') return renderDocumentAnalysis(value);
  if (Array.isArray(value)) {
    return `<ul>${value.map((item) => `<li>${renderStructuredDocumentAnalysisValue(item)}</li>`).join('')}</ul>`;
  }
  if (typeof value === 'object') {
    return `<dl class="document-analysis-fields">${Object.entries(value).map(([key, item]) => `<div><dt>${escapeHtml(formatAnalysisKey(key))}</dt><dd>${renderStructuredDocumentAnalysisValue(item)}</dd></div>`).join('')}</dl>`;
  }
  return `<p>${escapeHtml(String(value))}</p>`;
}

function renderStructuredDocumentAnalysisValue(value) {
  if (value == null || value === '') return '<span class="muted">Niet gevonden</span>';
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return escapeHtml(String(value));
  return renderStructuredDocumentAnalysis(value);
}

function formatAnalysisKey(value) {
  return String(value).replace(/[_-]+/g, ' ').replace(/\b\w/g, (character) => character.toUpperCase());
}

function renderDocumentAnalysis(text) {
  const lines = String(text).replace(/\r\n?/g, '\n').split('\n');
  const blocks = [];
  let index = 0;
  while (index < lines.length) {
    const line = lines[index].trim();
    if (!line) {
      index += 1;
      continue;
    }
    if (/^#{1,3}\s+/.test(line)) {
      blocks.push(`<h4>${formatDocumentAnalysisInline(line.replace(/^#{1,3}\s+/, ''))}</h4>`);
      index += 1;
      continue;
    }
    if (/^\*\*[^*]+\*\*:?$/.test(line)) {
      blocks.push(`<h4>${formatDocumentAnalysisInline(line.replace(/^\*\*|\*\*:?$/g, ''))}</h4>`);
      index += 1;
      continue;
    }
    if (isDocumentAnalysisHeading(line)) {
      blocks.push(`<h4>${formatDocumentAnalysisInline(line.replace(/^[^\p{L}\p{N}]+/u, ''))}</h4>`);
      index += 1;
      continue;
    }
    if (isDocumentAnalysisTableLine(lines[index]) && isDocumentAnalysisTableLine(lines[index + 1] || '')) {
      const tableLines = [];
      while (index < lines.length && isDocumentAnalysisTableLine(lines[index])) {
        tableLines.push(lines[index]);
        index += 1;
      }
      blocks.push(renderDocumentAnalysisTable(tableLines));
      continue;
    }
    if (/^([-*]|\d+\.)\s+/.test(line)) {
      const items = [];
      while (index < lines.length && /^([-*]|\d+\.)\s+/.test(lines[index].trim())) {
        items.push(lines[index].trim().replace(/^([-*]|\d+\.)\s+/, ''));
        index += 1;
      }
      blocks.push(`<ul>${items.map((item) => `<li>${formatDocumentAnalysisInline(item)}</li>`).join('')}</ul>`);
      continue;
    }
    const paragraph = [line];
    index += 1;
    while (index < lines.length && lines[index].trim() && !isDocumentAnalysisBlockStart(lines[index])) {
      paragraph.push(lines[index].trim());
      index += 1;
    }
    blocks.push(`<p>${formatDocumentAnalysisInline(paragraph.join(' '))}</p>`);
  }
  return blocks.join('');
}

function isDocumentAnalysisBlockStart(line) {
  const value = String(line || '').trim();
  return /^#{1,3}\s+/.test(value) || /^\*\*[^*]+\*\*:?$/.test(value) || isDocumentAnalysisHeading(value) || isDocumentAnalysisTableLine(value) || /^([-*]|\d+\.)\s+/.test(value);
}

function isDocumentAnalysisHeading(line) {
  const value = String(line || '').trim().replace(/^[^\p{L}\p{N}]+/u, '').replace(/:$/, '');
  if (!value || value.length > 70 || value.includes('.')) return false;
  return /^(belangrijkste gegevens|kerngegevens|apparaatgegevens|objectgegevens|meetresultaten|bevindingen|conclusie|wat betekent dit voor jou|partijen|data|bedragen|risico|risico's|gebreken)/i.test(value);
}

function isDocumentAnalysisTableLine(line) {
  const value = String(line || '').trim();
  return (value.includes('|') && value.split('|').filter((cell) => cell.trim()).length >= 2) || value.split('\t').filter(Boolean).length >= 2;
}

function renderDocumentAnalysisTable(lines) {
  const rows = lines
    .map((line) => line.includes('|') ? line.split('|').map((cell) => cell.trim()).filter(Boolean) : line.split('\t').map((cell) => cell.trim()).filter(Boolean))
    .filter((row) => row.length && !row.every((cell) => /^:?-{3,}:?$/.test(cell)));
  if (!rows.length) return '';
  const [head, ...body] = rows;
  return `<div class="document-analysis-table"><table><thead><tr>${head.map((cell) => `<th>${formatDocumentAnalysisInline(cell)}</th>`).join('')}</tr></thead><tbody>${body.map((row) => `<tr>${head.map((_, cellIndex) => `<td>${formatDocumentAnalysisInline(row[cellIndex] || '')}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`;
}

function formatDocumentAnalysisInline(value) {
  return escapeHtml(value).replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
}

function renderExistingDocumentOptions(allDocuments, linkedDocuments) {
  const linkedIDs = new Set(linkedDocuments.map((document) => document.id));
  const available = allDocuments.filter((document) => !linkedIDs.has(document.id));
  const select = document.querySelector('#document-link-form select[name="documentId"]');
  select.innerHTML = available.length
    ? `<option value="">Kies een document</option>${available.map((document) => `<option value="${document.id}">${escapeHtml(document.originalFilename)}</option>`).join('')}`
    : '<option value="">Geen andere documenten beschikbaar</option>';
  select.disabled = !available.length;
  document.querySelector('#document-link-form button').disabled = !available.length;
}

async function uploadTransactionDocument(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const dialog = document.querySelector('#transaction-documents-dialog');
  const transactionID = dialog.dataset.transactionId;
  setDocumentStatus(transactionID, 'Document uploaden...');
  try {
    const document = await createDocument(form);
    const linkResponse = await linkDocumentToTransaction(transactionID, document.id);
    if (!linkResponse.ok) {
      const error = await linkResponse.json();
      setDocumentStatus(transactionID, error.error || 'Document opgeslagen, maar koppelen mislukt');
      return;
    }
    form.reset();
    setDocumentStatus(transactionID, 'Document geupload en gekoppeld.');
    if (dialog.dataset.transactionId === transactionID) {
      await loadTransactionDocuments();
    }
  } catch (error) {
    setDocumentStatus(transactionID, error.message || 'Uploaden mislukt');
  }
}

async function createDocument(form) {
  const response = await fetch(`${apiBaseUrl}/api/v1/documents`, { method: 'POST', body: new FormData(form) });
  const document = await response.json();
  if (!response.ok) {
    throw new Error(document.error || 'Uploaden mislukt');
  }
  return document;
}

async function linkExistingDocument(event) {
  event.preventDefault();
  const transactionID = document.querySelector('#transaction-documents-dialog').dataset.transactionId;
  const documentID = Number(new FormData(event.currentTarget).get('documentId'));
  if (!documentID) {
    return;
  }
  try {
    const response = await linkDocumentToTransaction(transactionID, documentID);
    if (!response.ok) {
      const error = await response.json();
      setDocumentStatus(transactionID, error.error || 'Koppelen mislukt');
      return;
    }
    setDocumentStatus(transactionID, 'Document gekoppeld.');
    if (document.querySelector('#transaction-documents-dialog').dataset.transactionId === transactionID) {
      await loadTransactionDocuments();
    }
  } catch (error) {
    setDocumentStatus(transactionID, error.message || 'Koppelen mislukt');
  }
}

function linkDocumentToTransaction(transactionID, documentID) {
  return fetch(`${apiBaseUrl}/api/v1/transactions/${transactionID}/documents`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ documentId: documentID }),
  });
}

async function unlinkTransactionDocument(event) {
  const button = event.target.closest('button[data-action="unlink-document"]');
  if (!button) {
    return;
  }
  const transactionID = document.querySelector('#transaction-documents-dialog').dataset.transactionId;
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/transactions/${transactionID}/documents/${button.dataset.documentId}`, { method: 'DELETE' });
    if (!response.ok) {
      const error = await response.json();
      setDocumentStatus(transactionID, error.error || 'Koppeling verwijderen mislukt');
      return;
    }
    setDocumentStatus(transactionID, 'Koppeling verwijderd; het document blijft bewaard.');
    if (document.querySelector('#transaction-documents-dialog').dataset.transactionId === transactionID) {
      await loadTransactionDocuments();
    }
  } catch (error) {
    setDocumentStatus(transactionID, error.message || 'Koppeling verwijderen mislukt');
  }
}

function setDocumentStatus(transactionID, message) {
  const dialog = document.querySelector('#transaction-documents-dialog');
  if (dialog.dataset.transactionId === String(transactionID)) {
    document.querySelector('#document-dialog-status').textContent = message;
  }
}

function updateDocumentCount(transactionID, count) {
  document.querySelectorAll(`tr[data-transaction-id="${transactionID}"] [data-document-count]`).forEach((element) => {
    element.textContent = count > 0 ? String(count) : '';
  });
}

function formatFileSize(size) {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function assetTypeOptions(selected = '') {
  const types = [
    ['real_estate', 'Vastgoed'], ['bank_account', 'Bankrekening'], ['investment_account', 'Beleggingsrekening'],
    ['vehicle', 'Voertuig'], ['equipment', 'Materieel'], ['valuable', 'Waardevol bezit'], ['cash', 'Contant geld'], ['other', 'Overig'],
  ];
  return types.map(([value, label]) => `<option value="${value}" ${value === selected ? 'selected' : ''}>${label}</option>`).join('');
}

function assetStatusOptions(selected = '') {
  return [['planned', 'Gepland'], ['active', 'Actief'], ['disposed', 'Afgevoerd']]
    .map(([value, label]) => `<option value="${value}" ${value === selected ? 'selected' : ''}>${label}</option>`).join('');
}

function assetTypeLabel(value) {
  const labels = { real_estate: 'Vastgoed', bank_account: 'Bankrekening', investment_account: 'Beleggingsrekening', vehicle: 'Voertuig', equipment: 'Materieel', valuable: 'Waardevol bezit', cash: 'Contant geld', other: 'Overig' };
  return labels[value] || value;
}

function assetStatusLabel(value) {
  return { planned: 'Gepland', active: 'Actief', disposed: 'Afgevoerd' }[value] || value;
}

function assetValuationTypeOptions(selected = 'estimate') {
  return [['purchase', 'Aankoop'], ['estimate', 'Schatting'], ['appraisal', 'Taxatie'], ['tax', 'Fiscaal'], ['market', 'Marktwaarde'], ['other', 'Overig']]
    .map(([value, label]) => `<option value="${value}" ${value === selected ? 'selected' : ''}>${label}</option>`).join('');
}

function assetValuationTypeLabel(value) {
  return { purchase: 'Aankoop', estimate: 'Schatting', appraisal: 'Taxatie', tax: 'Fiscaal', market: 'Marktwaarde', other: 'Overig' }[value] || value;
}

async function loadAssets() {
  const form = document.querySelector('#asset-filter-form');
  const params = new URLSearchParams();
  if (form.elements.type.value) params.set('type', form.elements.type.value);
  if (form.elements.status.value) params.set('status', form.elements.status.value);
  const list = document.querySelector('#asset-list');
  list.innerHTML = '<p>Bezittingen laden...</p>';
  try {
    const [response, accountResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/assets?${params}`),
      fetch(`${apiBaseUrl}/api/v1/accounts`),
    ]);
    const [assets, accounts] = await Promise.all([response.json(), accountResponse.json()]);
    if (!response.ok) throw new Error(assets.error || 'Bezittingen laden mislukt');
    if (!accountResponse.ok) throw new Error(accounts.error || 'Spaarrekeningen laden mislukt');
    assetsState = assets;
    const includeSavings = (!form.elements.type.value || form.elements.type.value === 'bank_account')
      && (!form.elements.status.value || form.elements.status.value === 'active');
    assetListState = includeSavings ? [...assets, ...accounts.filter((account) => account.type === 'savings').map(savingsAccountAsset)] : assets;
    assetsLoaded = true;
    list.innerHTML = renderAssetList(assetListState);
  } catch (error) {
    list.innerHTML = `<p class="error">${escapeHtml(error.message)}</p>`;
  }
}

function renderAssetList(assets) {
  if (!assets.length) return '<p>Geen bezittingen gevonden.</p>';
  return `<ul>${assets.map((asset) => `
    <li><button type="button" class="asset-list-button ${(asset.source === 'savings-account' ? selectedSavingsAccountState?.id === asset.accountId : selectedAssetState?.id === asset.id) ? 'active' : ''}" ${asset.source === 'savings-account' ? `data-savings-account-id="${asset.accountId}"` : `data-asset-id="${asset.id}"`}><span><strong>${escapeHtml(asset.name)}</strong><small>${escapeHtml(asset.source === 'savings-account' ? 'Spaarrekening' : assetTypeLabel(asset.assetType))} · ${escapeHtml(assetStatusLabel(asset.status))}</small></span><b>${formatAssetValue(asset.currentValueCents, asset.currency)}</b></button></li>
  `).join('')}</ul>`;
}

function savingsAccountAsset(account) {
  return { source: 'savings-account', accountId: account.id, name: account.name, assetType: 'bank_account', status: 'active', currency: account.currency || 'EUR', currentValueCents: account.balanceCents, account };
}

async function handleAssetAction(event) {
  const savingsButton = event.target.closest('[data-savings-account-id]');
  if (savingsButton) {
    showSavingsAccountDetail(Number(savingsButton.dataset.savingsAccountId));
    return;
  }
  const selectButton = event.target.closest('[data-asset-id]');
  if (selectButton) {
    await loadAssetDetail(Number(selectButton.dataset.assetId));
    return;
  }
  const button = event.target.closest('[data-asset-action], [data-asset-detail-target]');
  if (!button) return;
  if (button.dataset.assetDetailTarget) {
    switchAssetDetailSection(button.dataset.assetDetailTarget);
    return;
  }
  switch (button.dataset.assetAction) {
  case 'new':
    showAssetForm();
    break;
  case 'edit':
    showAssetForm(selectedAssetState);
    break;
  case 'cancel-form':
    hideAssetForm();
    break;
  case 'delete':
    await deleteSelectedAsset();
    break;
  case 'open-savings-transactions':
    openSavingsAccountTransactions();
    break;
  case 'unlink-asset-document':
    await unlinkAssetDocument(Number(button.dataset.documentId));
    break;
  case 'search-transactions':
    await loadAssetTransactionCandidates();
    break;
  case 'unlink-asset-transaction':
    await unlinkAssetTransaction(Number(button.dataset.transactionId));
    break;
  case 'edit-valuation':
    editAssetValuation(Number(button.dataset.valuationId));
    break;
  case 'delete-valuation':
    await deleteAssetValuation(Number(button.dataset.valuationId));
    break;
  case 'cancel-valuation-edit':
    resetAssetValuationForm();
    break;
  case 'edit-contract':
    editAssetContract(Number(button.dataset.contractId));
    break;
  case 'open-contract':
    await openAssetContract(Number(button.dataset.contractId));
    break;
  case 'cancel-contract-edit':
    resetAssetContractForm();
    break;
  }
}

function showAssetForm(asset = null) {
  assetDetailRequestID += 1;
  selectedSavingsAccountState = null;
  const form = document.querySelector('#asset-form');
  form.reset();
  form.elements.id.value = asset?.id || '';
  form.elements.name.value = asset?.name || '';
  form.elements.assetType.value = asset?.assetType || 'real_estate';
  form.elements.status.value = asset?.status || 'active';
  form.elements.acquiredAt.value = asset?.acquiredAt || '';
  form.elements.disposedAt.value = asset?.disposedAt || '';
  form.elements.purchaseValue.value = centsToInput(asset?.purchaseValueCents || 0);
  form.elements.description.value = asset?.description || '';
  document.querySelector('#asset-form-title').textContent = asset ? 'Bezitting wijzigen' : 'Nieuwe bezitting';
  document.querySelector('#asset-form-panel').classList.remove('hidden');
  document.querySelector('#asset-detail').classList.add('hidden');
  document.querySelector('#savings-account-detail').classList.add('hidden');
  document.querySelector('#asset-empty-state').classList.add('hidden');
}

function showSavingsAccountDetail(accountID) {
  const item = assetListState.find((asset) => asset.source === 'savings-account' && asset.accountId === accountID);
  if (!item) return;
  assetDetailRequestID += 1;
  selectedAssetState = null;
  selectedSavingsAccountState = item.account;
  document.querySelector('#asset-form-panel').classList.add('hidden');
  document.querySelector('#asset-detail').classList.add('hidden');
  document.querySelector('#asset-empty-state').classList.add('hidden');
  const detail = document.querySelector('#savings-account-detail');
  detail.innerHTML = `
    <div class="asset-detail-heading"><div><small>Spaarrekening</small><h2>${escapeHtml(item.name)}</h2></div><button type="button" class="secondary-button" data-asset-action="open-savings-transactions">Bekijk transacties</button></div>
    <dl class="asset-overview-grid"><div><dt>Type</dt><dd>Spaarrekening</dd></div><div><dt>Status</dt><dd>Actief</dd></div><div><dt>Rekeninghouder</dt><dd>${escapeHtml(item.account.owner || 'Niet vastgelegd')}</dd></div><div><dt>IBAN</dt><dd>${escapeHtml(item.account.iban || 'Niet vastgelegd')}</dd></div><div><dt>Actuele waarde</dt><dd>${formatAssetValue(item.currentValueCents, item.currency)}</dd></div></dl>
    <p class="asset-description">De actuele waarde volgt automatisch uit het rekeningsaldo.</p>`;
  detail.classList.remove('hidden');
  document.querySelector('#asset-list').innerHTML = renderAssetList(assetListState);
}

function openSavingsAccountTransactions() {
  if (!selectedSavingsAccountState) return;
  activateMainSection('transactions');
  document.querySelector('#dashboard-account-select').value = String(selectedSavingsAccountState.id);
  accountTransactionPage.offset = 0;
  loadDashboardTransactions();
}

function hideAssetForm() {
  document.querySelector('#asset-form-panel').classList.add('hidden');
  if (selectedAssetState) {
    document.querySelector('#asset-detail').classList.remove('hidden');
  } else {
    document.querySelector('#asset-empty-state').classList.remove('hidden');
  }
}

async function saveAsset(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const id = Number(form.elements.id.value);
  const input = {
    name: form.elements.name.value,
    assetType: form.elements.assetType.value,
    status: form.elements.status.value,
    currency: selectedAssetState?.id === id ? selectedAssetState.currency : 'EUR',
    acquiredAt: form.elements.acquiredAt.value,
    disposedAt: form.elements.disposedAt.value,
    purchaseValueCents: amountInputToCents(form.elements.purchaseValue.value),
    description: form.elements.description.value,
  };
  try {
    const response = await fetch(id ? `${apiBaseUrl}/api/v1/assets/${id}` : `${apiBaseUrl}/api/v1/assets`, { method: id ? 'PATCH' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const asset = await response.json();
    if (!response.ok) throw new Error(asset.error || 'Bezitting opslaan mislukt');
    hideAssetForm();
    await loadAssets();
    await loadAssetDetail(asset.id);
  } catch (error) {
    window.alert(error.message);
  }
}

async function loadAssetDetail(id, targetSection = 'overview') {
  const requestID = ++assetDetailRequestID;
  selectedSavingsAccountState = null;
  try {
    const [assetResponse, valuationsResponse, documentsResponse, transactionsResponse, relationsResponse, allDocumentsResponse, casesResponse, eventsResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/assets/${id}`), fetch(`${apiBaseUrl}/api/v1/assets/${id}/valuations`),
      fetch(`${apiBaseUrl}/api/v1/assets/${id}/documents`), fetch(`${apiBaseUrl}/api/v1/assets/${id}/transactions`),
      fetch(`${apiBaseUrl}/api/v1/assets/${id}/relations`), fetch(`${apiBaseUrl}/api/v1/documents`),
      fetch(`${apiBaseUrl}/api/v1/cases?type=contract`), fetch(`${apiBaseUrl}/api/v1/assets/${id}/events`),
    ]);
    const [asset, valuations, documentsResult, transactions, relations, allDocumentsResult, contracts, eventsResult] = await Promise.all([assetResponse.json(), valuationsResponse.json(), documentsResponse.json(), transactionsResponse.json(), relationsResponse.json(), allDocumentsResponse.json(), casesResponse.json(), eventsResponse.json()]);
    const documentsUnavailable = documentsResponse.status === 501 || allDocumentsResponse.status === 501;
    if (!assetResponse.ok || !valuationsResponse.ok || (!documentsResponse.ok && documentsResponse.status !== 501) || !transactionsResponse.ok || !relationsResponse.ok || (!allDocumentsResponse.ok && allDocumentsResponse.status !== 501) || !casesResponse.ok || !eventsResponse.ok) throw new Error(asset.error || valuations.error || documentsResult.error || transactions.error || relations.error || allDocumentsResult.error || contracts.error || eventsResult?.error || 'Bezitting laden mislukt');
    const events = validateDomainEvents(eventsResult);
    if (requestID !== assetDetailRequestID) return;
    const documents = documentsResponse.status === 501 ? [] : documentsResult;
    const allDocuments = allDocumentsResponse.status === 501 ? [] : allDocumentsResult;
    selectedAssetState = asset;
    assetValuationsState = valuations;
    assetContractsState = contractsForRelations(contracts, relations, 'asset', asset.id);
    document.querySelector('#asset-detail-name').textContent = asset.name;
    document.querySelector('#asset-overview').innerHTML = renderAssetOverview(asset, { contracts: assetContractsState, documents, transactions, events });
    document.querySelector('#asset-current-value').textContent = formatAssetValue(asset.currentValueCents, asset.currency);
    document.querySelector('#asset-valuation-list').innerHTML = renderAssetValuations(valuations, asset.currency);
    resetAssetValuationForm();
    document.querySelector('#asset-contract-list').innerHTML = renderAssetContracts(assetContractsState);
    resetAssetContractForm();
    document.querySelector('#asset-document-list').innerHTML = renderAssetDocuments(documents);
    document.querySelector('#asset-document-status').textContent = documentsUnavailable ? 'Documentopslag is momenteel niet beschikbaar.' : '';
    renderAssetDocumentOptions(allDocuments, documents);
    document.querySelector('#asset-transaction-list').innerHTML = renderAssetTransactions(transactions);
    document.querySelector('#asset-timeline').innerHTML = renderDomainTimeline(events);
    document.querySelector('#asset-detail').classList.remove('hidden');
    document.querySelector('#savings-account-detail').classList.add('hidden');
    document.querySelector('#asset-empty-state').classList.add('hidden');
    document.querySelector('#asset-form-panel').classList.add('hidden');
    document.querySelector('#asset-list').innerHTML = renderAssetList(assetListState);
    switchAssetDetailSection(targetSection);
    await loadAssetTransactionCandidates();
  } catch (error) {
    if (requestID === assetDetailRequestID) window.alert(error.message);
  }
}

function contractsForRelations(contracts, relations, objectType, objectID) {
  const contractIDs = new Set(relations.filter((relation) => relation.relationType === 'relates_to' && (
    (relation.sourceType === 'case' && relation.targetType === objectType && relation.targetId === objectID)
    || (relation.targetType === 'case' && relation.sourceType === objectType && relation.sourceId === objectID)
  )).map((relation) => relation.sourceType === 'case' ? relation.sourceId : relation.targetId));
  return contracts.filter((item) => contractIDs.has(item.id));
}

function renderAssetOverview(asset, related = {}) {
  return `${renderCompactTimeline(related.events || [])}<dl class="asset-overview-grid"><div><dt>Type</dt><dd>${escapeHtml(assetTypeLabel(asset.assetType))}</dd></div><div><dt>Status</dt><dd>${escapeHtml(assetStatusLabel(asset.status))}</dd></div><div><dt>Aankoopdatum</dt><dd>${escapeHtml(asset.acquiredAt || 'Niet vastgelegd')}</dd></div><div><dt>Afvoerdatum</dt><dd>${escapeHtml(asset.disposedAt || 'Niet van toepassing')}</dd></div><div><dt>Aankoopwaarde</dt><dd>${formatAssetValue(asset.purchaseValueCents, asset.currency)}</dd></div><div><dt>Huidige waarde</dt><dd>${formatAssetValue(asset.currentValueCents, asset.currency)}</dd></div></dl><p class="asset-description">${escapeHtml(asset.description || 'Geen omschrijving.')}</p>${renderWealthOverviewRelated(related, 'asset')}`;
}

function renderCompactTimeline(events) {
  const sorted = [...events].sort((left, right) => String(left.eventDate).localeCompare(String(right.eventDate)) || left.id - right.id);
  const items = sorted.length <= 6 ? sorted : [sorted[0], ...sorted.slice(-5)];
  if (!items.length) return '<div class="asset-overview-summary compact-timeline"><h3>Tijdlijn</h3><p>Nog geen gebeurtenissen.</p></div>';
  return `<div class="asset-overview-summary compact-timeline"><div class="compact-timeline-heading"><h3>Tijdlijn</h3><span>${escapeHtml(formatTimelineDate(sorted[0].eventDate))} tot ${escapeHtml(formatTimelineDate(sorted[sorted.length - 1].eventDate))}</span></div><ol>${items.map((event, index) => `<li class="${index === 0 ? 'first' : ''} ${index === items.length - 1 ? 'last' : ''}"><button type="button" class="compact-timeline-node" aria-label="${escapeHtml(domainEventTypeLabel(event.eventType))} op ${escapeHtml(formatTimelineDate(event.eventDate))}"><span class="compact-timeline-dot"></span><time datetime="${escapeHtml(event.eventDate)}">${escapeHtml(formatTimelineDate(event.eventDate))}</time><strong>${escapeHtml(domainEventTypeLabel(event.eventType))}</strong><span class="compact-timeline-popover">${escapeHtml(event.description || domainEventTypeLabel(event.eventType))}${event.relatedEntityLabel ? `<br>${escapeHtml(event.relatedEntityLabel)}` : ''}</span></button></li>`).join('')}</ol></div>`;
}

function renderWealthOverviewRelated(related, type) {
  const actionPrefix = type === 'asset' ? 'asset' : 'liability';
  const detailPrefix = type === 'asset' ? 'asset' : 'liability';
  return `<div class="asset-overview-related">
    ${renderOverviewContracts(related.contracts || [], actionPrefix)}
    ${renderOverviewDocuments(related.documents || [])}
    ${renderOverviewTransactions(related.transactions || [])}
    <div class="overview-jump-links">
      <button type="button" class="secondary-button" data-${detailPrefix}-detail-target="contracts">Alle contracten</button>
      <button type="button" class="secondary-button" data-${detailPrefix}-detail-target="documents">Alle documenten</button>
      <button type="button" class="secondary-button" data-${detailPrefix}-detail-target="transactions">Alle transacties</button>
    </div>
  </div>`;
}

function renderOverviewContracts(contracts, actionPrefix) {
  const items = contracts.slice(0, 3);
  return `<section class="asset-overview-summary"><h3>Contracten</h3>${items.length ? `<ul>${items.map((item) => `<li><button type="button" class="link-button" data-${actionPrefix}-action="open-contract" data-contract-id="${item.id}">${escapeHtml(item.title)}</button><small>${escapeHtml(caseStatusLabel(item.status))}${item.endDate ? ` · t/m ${escapeHtml(item.endDate)}` : ''}</small></li>`).join('')}</ul>` : '<p>Nog geen contracten gekoppeld.</p>'}</section>`;
}

function renderOverviewDocuments(documents) {
  const items = documents.slice(0, 3);
  return `<section class="asset-overview-summary"><h3>Documenten</h3>${items.length ? `<ul>${items.map((item) => `<li><a href="${apiBaseUrl}/api/v1/documents/${item.id}/content" target="_blank" rel="noopener">${escapeHtml(item.originalFilename)}</a><small>${escapeHtml(item.documentDate || item.mimeType || '')}${item.translationStatus && item.translationStatus !== 'not_requested' ? ` · ${escapeHtml(documentTranslationStatusLabel(item.translationStatus))}` : ''}</small></li>`).join('')}</ul>` : '<p>Nog geen documenten gekoppeld.</p>'}</section>`;
}

function renderOverviewTransactions(transactions) {
  const items = transactions.slice(0, 5);
  return `<section class="asset-overview-summary"><h3>Transacties</h3>${items.length ? `<ul>${items.map((item) => `<li><span>${escapeHtml(item.description)}</span><small>${escapeHtml(item.bookedOn)}${item.counterparty ? ` · ${escapeHtml(item.counterparty)}` : ''}</small><strong>${formatAmount(item.amountCents)}</strong></li>`).join('')}</ul>` : '<p>Nog geen transacties gekoppeld.</p>'}</section>`;
}

function documentTranslationStatusLabel(value) {
  return { pending: 'analyse loopt', completed: 'geanalyseerd', unsupported: 'geen PDF', failed: 'analyse mislukt' }[value] || value;
}

function renderAssetValuations(valuations, currency) {
  if (!valuations.length) return '<p>Nog geen waarderingen. De actuele waarde is daarom 0.</p>';
  return `<ol>${valuations.map((valuation) => `
    <li><div><strong>${formatAssetValue(valuation.valueCents, currency)}</strong><small>${escapeHtml(valuation.valuationDate)} &middot; ${escapeHtml(assetValuationTypeLabel(valuation.valuationType))}</small>${valuation.source ? `<span>Bron: ${escapeHtml(valuation.source)}</span>` : ''}${valuation.notes ? `<p>${escapeHtml(valuation.notes)}</p>` : ''}</div><div class="row-actions"><button type="button" class="icon-button" data-asset-action="edit-valuation" data-valuation-id="${valuation.id}" aria-label="Wijzig waardering">&#9998;</button><button type="button" class="icon-button danger" data-asset-action="delete-valuation" data-valuation-id="${valuation.id}" aria-label="Verwijder waardering">&#10005;</button></div></li>
  `).join('')}</ol>`;
}

async function saveAssetValuation(event) {
  event.preventDefault();
  if (!selectedAssetState) return;
  const form = event.currentTarget;
  const valuationID = Number(form.elements.id.value);
  const assetID = selectedAssetState.id;
  const input = {
    valuationDate: form.elements.valuationDate.value,
    valueCents: amountInputToCents(form.elements.value.value),
    valuationType: form.elements.valuationType.value,
    source: form.elements.source.value,
    notes: form.elements.notes.value,
  };
  try {
    const response = await fetch(valuationID ? `${apiBaseUrl}/api/v1/assets/${assetID}/valuations/${valuationID}` : `${apiBaseUrl}/api/v1/assets/${assetID}/valuations`, {
      method: valuationID ? 'PATCH' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input),
    });
    const valuation = await response.json();
    if (!response.ok) throw new Error(valuation.error || 'Waardering opslaan mislukt');
    await loadAssets();
    await loadAssetDetail(assetID, 'value');
  } catch (error) {
    window.alert(error.message);
  }
}

function editAssetValuation(id) {
  const valuation = assetValuationsState.find((item) => item.id === id);
  if (!valuation) return;
  const form = document.querySelector('#asset-valuation-form');
  form.elements.id.value = valuation.id;
  form.elements.valuationDate.value = valuation.valuationDate;
  form.elements.value.value = centsToInput(valuation.valueCents);
  form.elements.valuationType.value = valuation.valuationType;
  form.elements.source.value = valuation.source || '';
  form.elements.notes.value = valuation.notes || '';
  form.querySelector('[data-asset-action="cancel-valuation-edit"]').classList.remove('hidden');
  switchAssetDetailSection('value');
}

function resetAssetValuationForm() {
  const form = document.querySelector('#asset-valuation-form');
  form.reset();
  form.elements.id.value = '';
  form.elements.valuationDate.value = localDateValue();
  form.elements.valuationType.value = 'estimate';
  form.querySelector('[data-asset-action="cancel-valuation-edit"]').classList.add('hidden');
}

function renderAssetContracts(contracts) {
  if (!contracts.length) return '<p>Nog geen contracten gekoppeld.</p>';
  return `<ul>${contracts.map((item) => `<li><div><strong>${escapeHtml(item.title)}</strong><small>${escapeHtml(caseStatusLabel(item.status))}${item.startDate ? ` &middot; vanaf ${escapeHtml(item.startDate)}` : ''}${item.endDate ? ` t/m ${escapeHtml(item.endDate)}` : ''}</small>${item.termMonths ? `<span>Looptijd: ${escapeHtml(String(item.termMonths))} maanden</span>` : ''}${item.description ? `<p>${escapeHtml(item.description)}</p>` : ''}</div><div class="row-actions"><button type="button" class="secondary-button" data-asset-action="open-contract" data-contract-id="${item.id}">Openen</button><button type="button" class="icon-button" data-asset-action="edit-contract" data-contract-id="${item.id}" aria-label="Wijzig contract">&#9998;</button></div></li>`).join('')}</ul>`;
}

function resetAssetContractForm() {
  const form = document.querySelector('#asset-contract-form');
  form.reset();
  form.elements.id.value = '';
  form.elements.status.value = 'active';
  form.querySelector('[data-asset-action="cancel-contract-edit"]').classList.add('hidden');
}

function editAssetContract(id) {
  const item = assetContractsState.find((contract) => contract.id === id);
  if (!item) return;
  const form = document.querySelector('#asset-contract-form');
  form.elements.id.value = item.id;
  form.elements.title.value = item.title;
  form.elements.description.value = item.description || '';
  form.elements.startDate.value = item.startDate || '';
  form.elements.endDate.value = item.endDate || '';
  form.elements.termMonths.value = item.termMonths || '';
  form.elements.status.value = item.status || 'active';
  form.querySelector('[data-asset-action="cancel-contract-edit"]').classList.remove('hidden');
  switchAssetDetailSection('contracts');
}

async function saveAssetContract(event) {
  event.preventDefault();
  if (!selectedAssetState) return;
  const form = event.currentTarget;
  const assetID = selectedAssetState.id;
  const contractID = Number(form.elements.id.value);
  const input = {
    caseType: 'contract',
    title: form.elements.title.value,
    description: form.elements.description.value,
    startDate: form.elements.startDate.value,
    endDate: form.elements.endDate.value,
    termMonths: Number(form.elements.termMonths.value),
    status: form.elements.status.value,
  };
  try {
    const response = await fetch(contractID ? `${apiBaseUrl}/api/v1/cases/${contractID}` : `${apiBaseUrl}/api/v1/cases`, { method: contractID ? 'PATCH' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const item = await response.json();
    if (!response.ok) throw new Error(item.error || 'Contract opslaan mislukt');
    if (!contractID) {
      const relationResponse = await fetch(`${apiBaseUrl}/api/v1/assets/${assetID}/relations`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ targetType: 'case', targetId: item.id, relationType: 'relates_to' }) });
      if (!relationResponse.ok) throw new Error((await relationResponse.json()).error || 'Contract opgeslagen, maar koppelen aan bezitting mislukt');
    }
    await loadAssetDetail(assetID, 'contracts');
  } catch (error) {
    window.alert(error.message);
  }
}

async function openAssetContract(id) {
  const form = document.querySelector('#case-filter-form');
  form.elements.caseType.value = 'contract';
  form.elements.status.value = '';
  activateMainSection('cases');
  await loadCases();
  await loadCaseDetail(id);
}

async function deleteAssetValuation(id) {
  if (!selectedAssetState || !window.confirm('Deze waardering verwijderen?')) return;
  const assetID = selectedAssetState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/assets/${assetID}/valuations/${id}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Waardering verwijderen mislukt');
  await loadAssets();
  await loadAssetDetail(assetID, 'value');
}

function switchAssetDetailSection(target) {
  document.querySelectorAll('[data-asset-detail-target]').forEach((button) => button.classList.toggle('active', button.dataset.assetDetailTarget === target));
  document.querySelectorAll('[data-asset-detail-section]').forEach((section) => section.classList.toggle('active', section.dataset.assetDetailSection === target));
}

async function deleteSelectedAsset() {
  if (!selectedAssetState || !window.confirm(`Bezitting "${selectedAssetState.name}" verwijderen?`)) return;
  const response = await fetch(`${apiBaseUrl}/api/v1/assets/${selectedAssetState.id}`, { method: 'DELETE' });
  if (!response.ok) {
    const error = await response.json();
    window.alert(error.error || 'Bezitting verwijderen mislukt');
    return;
  }
  selectedAssetState = null;
  document.querySelector('#asset-detail').classList.add('hidden');
  document.querySelector('#asset-empty-state').classList.remove('hidden');
  await loadAssets();
}

function renderAssetDocuments(documents) {
  if (!documents.length) return '<p>Nog geen documenten gekoppeld.</p>';
  return `<ul>${documents.map((document) => `<li><div><a href="${apiBaseUrl}/api/v1/documents/${document.id}/content" target="_blank" rel="noopener">${escapeHtml(document.originalFilename)}</a><small>${escapeHtml(document.documentDate || document.mimeType)} &middot; ${formatFileSize(document.sizeBytes)}</small>${renderDocumentTranslation(document)}</div><button type="button" class="icon-button danger" data-asset-action="unlink-asset-document" data-document-id="${document.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
}

function renderAssetDocumentOptions(allDocuments, linkedDocuments) {
  const linkedIDs = new Set(linkedDocuments.map((document) => document.id));
  const available = allDocuments.filter((document) => !linkedIDs.has(document.id));
  const select = document.querySelector('#asset-document-link-form select[name="documentId"]');
  select.innerHTML = available.length ? `<option value="">Kies een document</option>${available.map((document) => `<option value="${document.id}">${escapeHtml(document.originalFilename)}</option>`).join('')}` : '<option value="">Geen andere documenten beschikbaar</option>';
  select.disabled = !available.length;
  document.querySelector('#asset-document-link-form button').disabled = !available.length;
}

async function uploadAssetDocument(event) {
  event.preventDefault();
  if (!selectedAssetState) return;
  const form = event.currentTarget;
  const assetID = selectedAssetState.id;
  const status = document.querySelector('#asset-document-status');
  status.textContent = 'Document uploaden...';
  try {
    const document = await createDocument(form);
    const response = await linkDocumentToAsset(assetID, document.id);
    if (!response.ok) throw new Error((await response.json()).error || 'Document opgeslagen, maar koppelen mislukt');
    form.reset();
    status.textContent = 'Document geupload en gekoppeld.';
    if (selectedAssetState?.id === assetID) await loadAssetDetail(assetID, 'documents');
  } catch (error) {
    status.textContent = error.message || 'Uploaden mislukt';
  }
}

async function linkExistingAssetDocument(event) {
  event.preventDefault();
  if (!selectedAssetState) return;
  const assetID = selectedAssetState.id;
  const documentID = Number(new FormData(event.currentTarget).get('documentId'));
  if (!documentID) return;
  const response = await linkDocumentToAsset(assetID, documentID);
  if (!response.ok) return window.alert((await response.json()).error || 'Document koppelen mislukt');
  await loadAssetDetail(assetID, 'documents');
}

function linkDocumentToAsset(assetID, documentID) {
  return fetch(`${apiBaseUrl}/api/v1/assets/${assetID}/documents`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ documentId: documentID }) });
}

async function unlinkAssetDocument(documentID) {
  if (!selectedAssetState) return;
  const assetID = selectedAssetState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/assets/${assetID}/documents/${documentID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Document ontkoppelen mislukt');
  await loadAssetDetail(assetID, 'documents');
}

async function loadAssetTransactionCandidates() {
  if (!selectedAssetState) return;
  const assetID = selectedAssetState.id;
  const form = document.querySelector('#asset-transaction-link-form');
  const search = form.elements.search.value.trim();
  const params = new URLSearchParams({ limit: '100' });
  if (search) params.set('search', search);
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/transactions?${params}`);
    const transactions = await response.json();
    if (!response.ok) throw new Error(transactions.error || 'Transacties zoeken mislukt');
    if (selectedAssetState?.id !== assetID) return;
    const select = form.elements.transactionId;
    select.innerHTML = transactions.length ? `<option value="">Kies een transactie</option>${transactions.map((transaction) => `<option value="${transaction.id}">${escapeHtml(transaction.bookedOn)} · ${escapeHtml(transaction.description)} · ${formatAmount(transaction.amountCents)}</option>`).join('')}` : '<option value="">Geen transacties gevonden</option>';
    select.disabled = !transactions.length;
  } catch (error) {
    if (selectedAssetState?.id === assetID) window.alert(error.message);
  }
}

async function linkAssetTransaction(event) {
  event.preventDefault();
  if (!selectedAssetState) return;
  const transactionID = Number(new FormData(event.currentTarget).get('transactionId'));
  if (!transactionID) return;
  const assetID = selectedAssetState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/assets/${assetID}/transactions`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ transactionId: transactionID }) });
  if (!response.ok) return window.alert((await response.json()).error || 'Transactie koppelen mislukt');
  await loadAssetDetail(assetID, 'transactions');
}

function renderAssetTransactions(transactions) {
  if (!transactions.length) return '<p>Nog geen transacties gekoppeld.</p>';
  return `<ul>${transactions.map((transaction) => `<li><div><strong>${escapeHtml(transaction.description)}</strong><small>${escapeHtml(transaction.bookedOn)} · ${escapeHtml(transaction.counterparty || '')}</small></div><span class="amount">${formatAmount(transaction.amountCents)}</span><button type="button" class="icon-button danger" data-asset-action="unlink-asset-transaction" data-transaction-id="${transaction.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
}

async function unlinkAssetTransaction(transactionID) {
  if (!selectedAssetState) return;
  const assetID = selectedAssetState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/assets/${assetID}/transactions/${transactionID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Transactie ontkoppelen mislukt');
  await loadAssetDetail(assetID, 'transactions');
}

function entityTypeLabel(value) {
  const labels = { asset: 'Bezitting', liability: 'Schuld', transaction: 'Transactie', document: 'Document', case: 'Contract' };
  return Object.hasOwn(labels, value) ? labels[value] : value;
}

function relationTypeLabel(value) {
  return { relates_to: 'Heeft betrekking op', evidence_for: 'Bewijs voor', financed_by: 'Gefinancierd door' }[value] || value;
}

function formatAssetValue(cents, currency) {
  return new Intl.NumberFormat('nl-NL', { style: 'currency', currency: currency || 'EUR' }).format(cents / 100);
}

function dashboardAmountsByCurrency(items, amountKey) {
  return items.reduce((totals, item) => {
    const currency = (item.currency || 'EUR').toUpperCase();
    totals[currency] = (totals[currency] || 0) + Number(item[amountKey] || 0);
    return totals;
  }, {});
}

function renderDashboardWealthAmounts(totals, currencies) {
  return currencies.map((currency) => `<strong>${formatAssetValue(totals[currency] || 0, currency)}</strong>`).join('');
}

function renderDashboardWealth(assets, liabilities, reservations = []) {
  const assetTotals = dashboardAmountsByCurrency(assets, 'currentValueCents');
  const liabilityTotals = dashboardAmountsByCurrency(liabilities, 'currentBalanceCents');
  const reservationTotals = dashboardAmountsByCurrency(reservations, 'amountCents');
  const currencies = [...new Set([...Object.keys(assetTotals), ...Object.keys(liabilityTotals), ...Object.keys(reservationTotals)])].sort();
  if (!currencies.length) currencies.push('EUR');
  const netTotals = Object.fromEntries(currencies.map((currency) => [currency, (assetTotals[currency] || 0) - (liabilityTotals[currency] || 0)]));
  const netWorth = document.querySelector('#dashboard-net-worth');
  const summary = document.querySelector('#dashboard-wealth-summary');
  if (!netWorth || !summary) return;

  netWorth.textContent = currencies.length === 1 ? formatAssetValue(netTotals[currencies[0]], currencies[0]) : `${currencies.length} valuta`;
  summary.innerHTML = `
    <button type="button" class="dashboard-wealth-card" data-wealth-target="assets">
      <span>Bezittingen <small>${assets.length} actief</small></span>
      <span class="dashboard-wealth-values">${renderDashboardWealthAmounts(assetTotals, currencies)}</span>
    </button>
    <button type="button" class="dashboard-wealth-card" data-wealth-target="liabilities">
      <span>Schulden <small>${liabilities.length} actief</small></span>
      <span class="dashboard-wealth-values">${renderDashboardWealthAmounts(liabilityTotals, currencies)}</span>
    </button>
    <div class="dashboard-wealth-card dashboard-wealth-net">
      <span>Nettovermogen <small>Bezittingen min schulden</small></span>
      <span class="dashboard-wealth-values">${renderDashboardWealthAmounts(netTotals, currencies)}</span>
    </div>
    <button type="button" class="dashboard-wealth-card" data-wealth-target="reservations">
      <span>Reserveringen <small>${reservations.length} gepland</small></span>
      <span class="dashboard-wealth-values">${renderDashboardWealthAmounts(reservationTotals, currencies)}</span>
    </button>
  `;
}

async function loadDashboardWealth() {
  const summary = document.querySelector('#dashboard-wealth-summary');
  if (!summary) return;
  try {
    const [assetResponse, liabilityResponse, reservationResponse, accountResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/assets?status=active`),
      fetch(`${apiBaseUrl}/api/v1/liabilities?status=active`),
      fetch(`${apiBaseUrl}/api/v1/reservations`),
      fetch(`${apiBaseUrl}/api/v1/accounts`),
    ]);
    if (!assetResponse.ok || !liabilityResponse.ok || !reservationResponse.ok || !accountResponse.ok) throw new Error('Vermogen kon niet worden geladen');
    const [assets, liabilities, reservations, accounts] = await Promise.all([assetResponse.json(), liabilityResponse.json(), reservationResponse.json(), accountResponse.json()]);
    const savingsAccounts = accounts.filter((account) => account.type === 'savings').map(savingsAccountAsset);
    renderDashboardWealth([...assets, ...savingsAccounts], liabilities, reservations);
  } catch (error) {
    document.querySelector('#dashboard-net-worth').textContent = 'Niet beschikbaar';
    summary.innerHTML = `<p class="dashboard-wealth-error">${escapeHtml(error.message)}</p>`;
  }
}

function handleDashboardWealthNavigation(event) {
  const target = event.target.closest('[data-wealth-target]')?.dataset.wealthTarget;
  if (target) activateMainSection('wealth', target);
}

async function loadDashboardContracts() {
  const count = document.querySelector('#dashboard-contract-count');
  const panel = document.querySelector('#dashboard-contracts');
  if (!count || !panel) return;
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/cases?type=contract`);
    const contracts = await response.json();
    if (!response.ok) throw new Error(contracts.error || 'Contracten laden mislukt');
    const visibleContracts = contracts.filter((item) => item.status !== 'archived');
    count.textContent = `${visibleContracts.length} actief`;
    panel.innerHTML = renderDashboardContracts(visibleContracts);
  } catch (error) {
    count.textContent = 'Niet beschikbaar';
    panel.innerHTML = `<p class="error">${escapeHtml(error.message)}</p>`;
  }
}

function renderDashboardContracts(contracts) {
  const sorted = [...contracts].sort((left, right) => dashboardContractSortValue(left) - dashboardContractSortValue(right) || left.title.localeCompare(right.title, 'nl', { sensitivity: 'base' }));
  if (!sorted.length) return '<p>Geen actieve contracten.</p>';
  const today = localDateValue();
  return `<ol class="contract-timeline"><li class="contract-timeline-today"><span class="contract-timeline-dot"></span><div><strong>Vandaag</strong><time datetime="${today}">${today}</time></div></li>${sorted.slice(0, 8).map((item) => {
    const actionDate = dashboardContractActionDate(item);
    const overdue = actionDate && actionDate < localDateValue();
    return `<li class="contract-timeline-item ${overdue ? 'overdue' : ''}"><span class="contract-timeline-dot"></span><button type="button" class="contract-timeline-card" data-dashboard-contract-id="${item.id}"><span><strong>${escapeHtml(item.title)}</strong><small>${escapeHtml(caseStatusLabel(item.status))}${item.termMonths ? ` · ${escapeHtml(String(item.termMonths))} mnd` : ''}</small></span><time datetime="${escapeHtml(actionDate || '')}">${escapeHtml(actionDate ? dashboardContractDateLabel(actionDate, overdue) : 'Geen datum')}</time></button></li>`;
  }).join('')}</ol>`;
}

function dashboardContractSortValue(item) {
  const actionDate = dashboardContractActionDate(item);
  return actionDate ? Date.parse(`${actionDate}T00:00:00`) : Number.MAX_SAFE_INTEGER;
}

function dashboardContractActionDate(item) {
  if (item.endDate) return item.endDate;
  if (!item.startDate || !item.termMonths) return '';
  const date = new Date(`${item.startDate}T00:00:00`);
  if (Number.isNaN(date.getTime())) return '';
  date.setMonth(date.getMonth() + Number(item.termMonths));
  return localDateValue(date);
}

function dashboardContractDateLabel(actionDate, overdue) {
  return `${overdue ? 'Verlopen' : 'Actie'} ${actionDate}`;
}

async function handleDashboardContractNavigation(event) {
  const contractID = Number(event.target.closest('[data-dashboard-contract-id]')?.dataset.dashboardContractId);
  if (!contractID) return;
  const form = document.querySelector('#case-filter-form');
  form.elements.caseType.value = 'contract';
  form.elements.status.value = '';
  activateMainSection('cases');
  await loadCases();
  await loadCaseDetail(contractID);
}

function centsToInput(cents) {
  return (Number(cents || 0) / 100).toFixed(2);
}

function amountInputToCents(value) {
  return Math.round(Number(value || 0) * 100);
}

function localDateValue(date = new Date()) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function liabilityTypeOptions(selected = '') {
  return [
    ['mortgage', 'Hypotheek'], ['loan', 'Lening'], ['tax_debt', 'Belastingschuld'], ['credit', 'Krediet'], ['other', 'Overig'],
  ].map(([value, label]) => `<option value="${value}" ${value === selected ? 'selected' : ''}>${label}</option>`).join('');
}

function liabilityStatusOptions(selected = '') {
  return [['planned', 'Gepland'], ['active', 'Actief'], ['repaid', 'Afgelost'], ['cancelled', 'Geannuleerd']]
    .map(([value, label]) => `<option value="${value}" ${value === selected ? 'selected' : ''}>${label}</option>`).join('');
}

function liabilityTypeLabel(value) {
  return { mortgage: 'Hypotheek', loan: 'Lening', tax_debt: 'Belastingschuld', credit: 'Krediet', other: 'Overig' }[value] || value;
}

function liabilityStatusLabel(value) {
  return { planned: 'Gepland', active: 'Actief', repaid: 'Afgelost', cancelled: 'Geannuleerd' }[value] || value;
}

async function loadLiabilities() {
  const form = document.querySelector('#liability-filter-form');
  const params = new URLSearchParams();
  if (form.elements.type.value) params.set('type', form.elements.type.value);
  if (form.elements.status.value) params.set('status', form.elements.status.value);
  const list = document.querySelector('#liability-list');
  list.innerHTML = '<p>Schulden laden...</p>';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/liabilities?${params}`);
    const liabilities = await response.json();
    if (!response.ok) throw new Error(liabilities.error || 'Schulden laden mislukt');
    liabilitiesState = liabilities;
    liabilitiesLoaded = true;
    list.innerHTML = renderLiabilityList(liabilitiesState);
  } catch (error) {
    list.innerHTML = `<p class="error">${escapeHtml(error.message)}</p>`;
  }
}

function renderLiabilityList(liabilities) {
  if (!liabilities.length) return '<p>Geen schulden gevonden.</p>';
  return `<ul>${liabilities.map((liability) => `<li><button type="button" class="asset-list-button ${selectedLiabilityState?.id === liability.id ? 'active' : ''}" data-liability-id="${liability.id}"><span><strong>${escapeHtml(liability.name)}</strong><small>${escapeHtml(liabilityTypeLabel(liability.liabilityType))} · ${escapeHtml(liabilityStatusLabel(liability.status))}</small></span><b>${formatAssetValue(liability.currentBalanceCents, liability.currency)}</b></button></li>`).join('')}</ul>`;
}

async function handleLiabilityAction(event) {
  const selectButton = event.target.closest('[data-liability-id]');
  if (selectButton) {
    await loadLiabilityDetail(Number(selectButton.dataset.liabilityId));
    return;
  }
  const button = event.target.closest('[data-liability-action], [data-liability-detail-target]');
  if (!button) return;
  if (button.dataset.liabilityDetailTarget) {
    switchLiabilityDetailSection(button.dataset.liabilityDetailTarget);
    return;
  }
  switch (button.dataset.liabilityAction) {
  case 'new': showLiabilityForm(); break;
  case 'edit': showLiabilityForm(selectedLiabilityState); break;
  case 'cancel-form': hideLiabilityForm(); break;
  case 'delete': await deleteSelectedLiability(); break;
  case 'unlink-document': await unlinkLiabilityDocument(Number(button.dataset.documentId)); break;
  case 'search-transactions': await loadLiabilityTransactionCandidates(); break;
  case 'unlink-transaction': await unlinkLiabilityTransaction(Number(button.dataset.transactionId)); break;
  case 'edit-contract': editLiabilityContract(Number(button.dataset.contractId)); break;
  case 'open-contract': await openLiabilityContract(Number(button.dataset.contractId)); break;
  case 'cancel-contract-edit': resetLiabilityContractForm(); break;
  }
}

function showLiabilityForm(liability = null) {
  liabilityDetailRequestID += 1;
  const form = document.querySelector('#liability-form');
  form.reset();
  form.elements.id.value = liability?.id || '';
  form.elements.name.value = liability?.name || '';
  form.elements.liabilityType.value = liability?.liabilityType || 'mortgage';
  form.elements.description.value = liability?.description || '';
  form.elements.creditor.value = liability?.creditor || '';
  form.elements.startDate.value = liability?.startDate || '';
  form.elements.endDate.value = liability?.endDate || '';
  form.elements.originalAmount.value = centsToInput(liability?.originalAmountCents || 0);
  form.elements.currentBalance.value = centsToInput(liability?.currentBalanceCents || 0);
  form.elements.status.value = liability?.status || 'active';
  document.querySelector('#liability-form-title').textContent = liability ? 'Schuld wijzigen' : 'Nieuwe schuld';
  document.querySelector('#liability-form-panel').classList.remove('hidden');
  document.querySelector('#liability-detail').classList.add('hidden');
  document.querySelector('#liability-empty-state').classList.add('hidden');
}

function hideLiabilityForm() {
  document.querySelector('#liability-form-panel').classList.add('hidden');
  document.querySelector(selectedLiabilityState ? '#liability-detail' : '#liability-empty-state').classList.remove('hidden');
}

async function saveLiability(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const id = Number(form.elements.id.value);
  const input = {
    name: form.elements.name.value,
    liabilityType: form.elements.liabilityType.value,
    description: form.elements.description.value,
    creditor: form.elements.creditor.value,
    startDate: form.elements.startDate.value,
    endDate: form.elements.endDate.value,
    originalAmountCents: amountInputToCents(form.elements.originalAmount.value),
    currentBalanceCents: amountInputToCents(form.elements.currentBalance.value),
    currency: selectedLiabilityState?.id === id ? selectedLiabilityState.currency : 'EUR',
    status: form.elements.status.value,
  };
  try {
    const response = await fetch(id ? `${apiBaseUrl}/api/v1/liabilities/${id}` : `${apiBaseUrl}/api/v1/liabilities`, { method: id ? 'PATCH' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const liability = await response.json();
    if (!response.ok) throw new Error(liability.error || 'Schuld opslaan mislukt');
    hideLiabilityForm();
    await loadLiabilities();
    await loadLiabilityDetail(liability.id);
  } catch (error) {
    window.alert(error.message);
  }
}

async function loadLiabilityDetail(id, targetSection = 'overview') {
  const requestID = ++liabilityDetailRequestID;
  try {
    const [liabilityResponse, documentsResponse, transactionsResponse, relationsResponse, allDocumentsResponse, casesResponse, eventsResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/liabilities/${id}`), fetch(`${apiBaseUrl}/api/v1/liabilities/${id}/documents`),
      fetch(`${apiBaseUrl}/api/v1/liabilities/${id}/transactions`), fetch(`${apiBaseUrl}/api/v1/liabilities/${id}/relations`),
      fetch(`${apiBaseUrl}/api/v1/documents`), fetch(`${apiBaseUrl}/api/v1/cases?type=contract`), fetch(`${apiBaseUrl}/api/v1/liabilities/${id}/events`),
    ]);
    const [liability, documentsResult, transactions, relations, allDocumentsResult, contracts, eventsResult] = await Promise.all([liabilityResponse.json(), documentsResponse.json(), transactionsResponse.json(), relationsResponse.json(), allDocumentsResponse.json(), casesResponse.json(), eventsResponse.json()]);
    const documentsUnavailable = documentsResponse.status === 501 || allDocumentsResponse.status === 501;
    if (!liabilityResponse.ok || (!documentsResponse.ok && documentsResponse.status !== 501) || !transactionsResponse.ok || !relationsResponse.ok || (!allDocumentsResponse.ok && allDocumentsResponse.status !== 501) || !casesResponse.ok || !eventsResponse.ok) throw new Error(liability.error || documentsResult.error || transactions.error || relations.error || allDocumentsResult.error || contracts.error || eventsResult?.error || 'Schuld laden mislukt');
    const events = validateDomainEvents(eventsResult);
    if (requestID !== liabilityDetailRequestID) return;
    const documents = documentsResponse.status === 501 ? [] : documentsResult;
    const allDocuments = allDocumentsResponse.status === 501 ? [] : allDocumentsResult;
    selectedLiabilityState = liability;
    liabilityContractsState = contractsForRelations(contracts, relations, 'liability', liability.id);
    document.querySelector('#liability-detail-name').textContent = liability.name;
    document.querySelector('#liability-overview').innerHTML = renderLiabilityOverview(liability, { contracts: liabilityContractsState, documents, transactions, events });
    document.querySelector('#liability-contract-list').innerHTML = renderLiabilityContracts(liabilityContractsState);
    resetLiabilityContractForm();
    document.querySelector('#liability-document-list').innerHTML = renderLiabilityDocuments(documents);
    document.querySelector('#liability-document-status').textContent = documentsUnavailable ? 'Documentopslag is momenteel niet beschikbaar.' : '';
    renderLiabilityDocumentOptions(allDocuments, documents);
    document.querySelector('#liability-transaction-list').innerHTML = renderLiabilityTransactions(transactions);
    document.querySelector('#liability-timeline').innerHTML = renderDomainTimeline(events);
    document.querySelector('#liability-detail').classList.remove('hidden');
    document.querySelector('#liability-empty-state').classList.add('hidden');
    document.querySelector('#liability-form-panel').classList.add('hidden');
    document.querySelector('#liability-list').innerHTML = renderLiabilityList(liabilitiesState);
    switchLiabilityDetailSection(targetSection);
    await loadLiabilityTransactionCandidates();
  } catch (error) {
    if (requestID === liabilityDetailRequestID) window.alert(error.message);
  }
}

function renderLiabilityOverview(liability, related = {}) {
  return `${renderCompactTimeline(related.events || [])}<dl class="asset-overview-grid"><div><dt>Type</dt><dd>${escapeHtml(liabilityTypeLabel(liability.liabilityType))}</dd></div><div><dt>Status</dt><dd>${escapeHtml(liabilityStatusLabel(liability.status))}</dd></div><div><dt>Schuldeiser</dt><dd>${escapeHtml(liability.creditor || 'Niet vastgelegd')}</dd></div><div><dt>Looptijd</dt><dd>${escapeHtml(liability.startDate || '...')} t/m ${escapeHtml(liability.endDate || '...')}</dd></div><div><dt>Oorspronkelijk bedrag</dt><dd>${formatAssetValue(liability.originalAmountCents, liability.currency)}</dd></div><div><dt>Huidig saldo</dt><dd>${formatAssetValue(liability.currentBalanceCents, liability.currency)}</dd></div></dl><p class="asset-description">${escapeHtml(liability.description || 'Geen omschrijving.')}</p>${renderWealthOverviewRelated(related, 'liability')}`;
}

function renderLiabilityContracts(contracts) {
  if (!contracts.length) return '<p>Nog geen contracten gekoppeld.</p>';
  return `<ul>${contracts.map((item) => `<li><div><strong>${escapeHtml(item.title)}</strong><small>${escapeHtml(caseStatusLabel(item.status))}${item.startDate ? ` &middot; vanaf ${escapeHtml(item.startDate)}` : ''}${item.endDate ? ` t/m ${escapeHtml(item.endDate)}` : ''}</small>${item.termMonths ? `<span>Looptijd: ${escapeHtml(String(item.termMonths))} maanden</span>` : ''}${item.description ? `<p>${escapeHtml(item.description)}</p>` : ''}</div><div class="row-actions"><button type="button" class="secondary-button" data-liability-action="open-contract" data-contract-id="${item.id}">Openen</button><button type="button" class="icon-button" data-liability-action="edit-contract" data-contract-id="${item.id}" aria-label="Wijzig contract">&#9998;</button></div></li>`).join('')}</ul>`;
}

function resetLiabilityContractForm() {
  const form = document.querySelector('#liability-contract-form');
  form.reset();
  form.elements.id.value = '';
  form.elements.status.value = 'active';
  form.querySelector('[data-liability-action="cancel-contract-edit"]').classList.add('hidden');
}

function editLiabilityContract(id) {
  const item = liabilityContractsState.find((contract) => contract.id === id);
  if (!item) return;
  const form = document.querySelector('#liability-contract-form');
  form.elements.id.value = item.id;
  form.elements.title.value = item.title;
  form.elements.description.value = item.description || '';
  form.elements.startDate.value = item.startDate || '';
  form.elements.endDate.value = item.endDate || '';
  form.elements.termMonths.value = item.termMonths || '';
  form.elements.status.value = item.status || 'active';
  form.querySelector('[data-liability-action="cancel-contract-edit"]').classList.remove('hidden');
  switchLiabilityDetailSection('contracts');
}

async function saveLiabilityContract(event) {
  event.preventDefault();
  if (!selectedLiabilityState) return;
  const form = event.currentTarget;
  const liabilityID = selectedLiabilityState.id;
  const contractID = Number(form.elements.id.value);
  const input = {
    caseType: 'contract',
    title: form.elements.title.value,
    description: form.elements.description.value,
    startDate: form.elements.startDate.value,
    endDate: form.elements.endDate.value,
    termMonths: Number(form.elements.termMonths.value),
    status: form.elements.status.value,
  };
  try {
    const response = await fetch(contractID ? `${apiBaseUrl}/api/v1/cases/${contractID}` : `${apiBaseUrl}/api/v1/cases`, { method: contractID ? 'PATCH' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const item = await response.json();
    if (!response.ok) throw new Error(item.error || 'Contract opslaan mislukt');
    if (!contractID) {
      const relationResponse = await fetch(`${apiBaseUrl}/api/v1/liabilities/${liabilityID}/relations`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ targetType: 'case', targetId: item.id, relationType: 'relates_to' }) });
      if (!relationResponse.ok) throw new Error((await relationResponse.json()).error || 'Contract opgeslagen, maar koppelen aan schuld mislukt');
    }
    await loadLiabilityDetail(liabilityID, 'contracts');
  } catch (error) {
    window.alert(error.message);
  }
}

async function openLiabilityContract(id) {
  const form = document.querySelector('#case-filter-form');
  form.elements.caseType.value = 'contract';
  form.elements.status.value = '';
  activateMainSection('cases');
  await loadCases();
  await loadCaseDetail(id);
}

function switchLiabilityDetailSection(target) {
  document.querySelectorAll('[data-liability-detail-target]').forEach((button) => button.classList.toggle('active', button.dataset.liabilityDetailTarget === target));
  document.querySelectorAll('[data-liability-detail-section]').forEach((section) => section.classList.toggle('active', section.dataset.liabilityDetailSection === target));
}

async function deleteSelectedLiability() {
  if (!selectedLiabilityState || !window.confirm(`Schuld "${selectedLiabilityState.name}" verwijderen?`)) return;
  const response = await fetch(`${apiBaseUrl}/api/v1/liabilities/${selectedLiabilityState.id}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Schuld verwijderen mislukt');
  selectedLiabilityState = null;
  liabilityDetailRequestID += 1;
  document.querySelector('#liability-detail').classList.add('hidden');
  document.querySelector('#liability-empty-state').classList.remove('hidden');
  await loadLiabilities();
}

async function loadReservations() {
  const form = document.querySelector('#reservation-filter-form');
  const params = new URLSearchParams();
  if (form.elements.type.value) params.set('type', form.elements.type.value);
  const list = document.querySelector('#reservation-list');
  list.innerHTML = '<p>Reserveringen laden...</p>';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/reservations?${params}`);
    const reservations = await response.json();
    if (!response.ok) throw new Error(reservations.error || 'Reserveringen laden mislukt');
    reservationsState = reservations;
    reservationsLoaded = true;
    list.innerHTML = renderReservationList(reservations);
  } catch (error) {
    list.innerHTML = `<p class="error">${escapeHtml(error.message)}</p>`;
  }
}

function renderReservationList(reservations) {
  if (!reservations.length) return '<p>Geen reserveringen gevonden.</p>';
  return `<ul>${reservations.map((item) => `<li><button type="button" class="asset-list-button ${selectedReservationState?.id === item.id ? 'active' : ''}" data-reservation-id="${item.id}"><span><strong>${escapeHtml(item.name)}</strong><small>${escapeHtml(liabilityTypeLabel(item.reserveType))}${item.dueDate ? ` · ${escapeHtml(item.dueDate)}` : ''}</small></span><b>${formatAssetValue(item.amountCents, item.currency)}</b></button></li>`).join('')}</ul>`;
}

async function handleReservationAction(event) {
  const selectButton = event.target.closest('[data-reservation-id]');
  if (selectButton) {
    await loadReservationDetail(Number(selectButton.dataset.reservationId));
    return;
  }
  const button = event.target.closest('[data-reservation-action], [data-reservation-detail-target]');
  if (!button) return;
  if (button.dataset.reservationDetailTarget) {
    switchReservationDetailSection(button.dataset.reservationDetailTarget);
    return;
  }
  switch (button.dataset.reservationAction) {
  case 'new': showReservationForm(); break;
  case 'edit': showReservationForm(selectedReservationState); break;
  case 'cancel-form': hideReservationForm(); break;
  case 'delete': await deleteSelectedReservation(); break;
  case 'unlink-document': await unlinkReservationDocument(Number(button.dataset.documentId)); break;
  }
}

function showReservationForm(item = null) {
  reservationDetailRequestID += 1;
  const form = document.querySelector('#reservation-form');
  form.reset();
  form.elements.id.value = item?.id || '';
  form.elements.name.value = item?.name || '';
  form.elements.liabilityType.value = item?.reserveType || 'other';
  form.elements.description.value = item?.description || '';
  form.elements.endDate.value = item?.dueDate || '';
  form.elements.currentBalance.value = centsToInput(item?.amountCents || 0);
  document.querySelector('#reservation-form-title').textContent = item ? 'Reservering wijzigen' : 'Nieuwe reservering';
  document.querySelector('#reservation-form-panel').classList.remove('hidden');
  document.querySelector('#reservation-detail').classList.add('hidden');
  document.querySelector('#reservation-empty-state').classList.add('hidden');
}

function hideReservationForm() {
  document.querySelector('#reservation-form-panel').classList.add('hidden');
  document.querySelector(selectedReservationState ? '#reservation-detail' : '#reservation-empty-state').classList.remove('hidden');
}

async function saveReservation(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const id = Number(form.elements.id.value);
  const amountCents = amountInputToCents(form.elements.currentBalance.value);
  const input = {
    name: form.elements.name.value,
    reserveType: form.elements.liabilityType.value,
    description: form.elements.description.value,
    dueDate: form.elements.endDate.value,
    amountCents,
    currency: 'EUR',
  };
  try {
    const response = await fetch(id ? `${apiBaseUrl}/api/v1/reservations/${id}` : `${apiBaseUrl}/api/v1/reservations`, { method: id ? 'PATCH' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const reservation = await response.json();
    if (!response.ok) throw new Error(reservation.error || 'Reservering opslaan mislukt');
    hideReservationForm();
    await loadReservations();
    await loadReservationDetail(reservation.id);
    loadDashboardWealth();
  } catch (error) {
    window.alert(error.message);
  }
}

async function loadReservationDetail(id, targetSection = 'overview') {
  const requestID = ++reservationDetailRequestID;
  try {
    const [reservationResponse, documentsResponse, allDocumentsResponse, eventsResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/reservations/${id}`), fetch(`${apiBaseUrl}/api/v1/reservations/${id}/documents`),
      fetch(`${apiBaseUrl}/api/v1/documents`), fetch(`${apiBaseUrl}/api/v1/reservations/${id}/events`),
    ]);
    const [reservation, documentsResult, allDocumentsResult, eventsResult] = await Promise.all([reservationResponse.json(), documentsResponse.json(), allDocumentsResponse.json(), eventsResponse.json()]);
    const documentsUnavailable = documentsResponse.status === 501 || allDocumentsResponse.status === 501;
    if (!reservationResponse.ok || (!documentsResponse.ok && documentsResponse.status !== 501) || (!allDocumentsResponse.ok && allDocumentsResponse.status !== 501) || !eventsResponse.ok) throw new Error(reservation.error || documentsResult.error || allDocumentsResult.error || eventsResult?.error || 'Reservering laden mislukt');
    const events = validateDomainEvents(eventsResult);
    if (requestID !== reservationDetailRequestID) return;
    const documents = documentsResponse.status === 501 ? [] : documentsResult;
    const allDocuments = allDocumentsResponse.status === 501 ? [] : allDocumentsResult;
    selectedReservationState = reservation;
    document.querySelector('#reservation-detail-name').textContent = reservation.name;
    document.querySelector('#reservation-overview').innerHTML = renderReservationOverview(reservation);
    document.querySelector('#reservation-document-list').innerHTML = renderReservationDocuments(documents);
    document.querySelector('#reservation-document-status').textContent = documentsUnavailable ? 'Documentopslag is momenteel niet beschikbaar.' : '';
    renderReservationDocumentOptions(allDocuments, documents);
    document.querySelector('#reservation-timeline').innerHTML = renderDomainTimeline(events);
    document.querySelector('#reservation-detail').classList.remove('hidden');
    document.querySelector('#reservation-empty-state').classList.add('hidden');
    document.querySelector('#reservation-form-panel').classList.add('hidden');
    document.querySelector('#reservation-list').innerHTML = renderReservationList(reservationsState);
    switchReservationDetailSection(targetSection);
  } catch (error) {
    if (requestID === reservationDetailRequestID) window.alert(error.message);
  }
}

function renderReservationOverview(item) {
  return `<dl class="asset-overview-grid"><div><dt>Type</dt><dd>${escapeHtml(liabilityTypeLabel(item.reserveType))}</dd></div><div><dt>Verwachte datum</dt><dd>${escapeHtml(item.dueDate || 'Niet vastgelegd')}</dd></div><div><dt>Bedrag</dt><dd>${formatAssetValue(item.amountCents, item.currency)}</dd></div></dl><p class="asset-description">${escapeHtml(item.description || 'Geen omschrijving.')}</p>`;
}

function switchReservationDetailSection(target) {
  document.querySelectorAll('[data-reservation-detail-target]').forEach((button) => button.classList.toggle('active', button.dataset.reservationDetailTarget === target));
  document.querySelectorAll('[data-reservation-detail-section]').forEach((section) => section.classList.toggle('active', section.dataset.reservationDetailSection === target));
}

async function deleteSelectedReservation() {
  if (!selectedReservationState || !window.confirm(`Reservering "${selectedReservationState.name}" verwijderen?`)) return;
  const response = await fetch(`${apiBaseUrl}/api/v1/reservations/${selectedReservationState.id}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Reservering verwijderen mislukt');
  selectedReservationState = null;
  reservationDetailRequestID += 1;
  document.querySelector('#reservation-detail').classList.add('hidden');
  document.querySelector('#reservation-empty-state').classList.remove('hidden');
  await loadReservations();
  loadDashboardWealth();
}

function renderReservationDocuments(documents) {
  if (!documents.length) return '<p>Nog geen documenten gekoppeld.</p>';
  return `<ul>${documents.map((item) => `<li><div><a href="${apiBaseUrl}/api/v1/documents/${item.id}/content" target="_blank" rel="noopener">${escapeHtml(item.originalFilename)}</a><small>${escapeHtml(item.documentDate || item.mimeType)} &middot; ${formatFileSize(item.sizeBytes)}</small>${renderDocumentTranslation(item)}</div><button type="button" class="icon-button danger" data-reservation-action="unlink-document" data-document-id="${item.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
}

function renderReservationDocumentOptions(allDocuments, linkedDocuments) {
  const linkedIDs = new Set(linkedDocuments.map((item) => item.id));
  const available = allDocuments.filter((item) => !linkedIDs.has(item.id));
  const form = document.querySelector('#reservation-document-link-form');
  form.elements.documentId.innerHTML = available.length ? `<option value="">Kies een document</option>${available.map((item) => `<option value="${item.id}">${escapeHtml(item.originalFilename)}</option>`).join('')}` : '<option value="">Geen andere documenten beschikbaar</option>';
  form.elements.documentId.disabled = !available.length;
  form.querySelector('button').disabled = !available.length;
}

function linkDocumentToReservation(reservationID, documentID) {
  return fetch(`${apiBaseUrl}/api/v1/reservations/${reservationID}/documents`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ documentId: documentID }) });
}

async function uploadReservationDocument(event) {
  event.preventDefault();
  if (!selectedReservationState) return;
  const form = event.currentTarget;
  const reservationID = selectedReservationState.id;
  const status = document.querySelector('#reservation-document-status');
  status.textContent = 'Document uploaden...';
  try {
    const created = await createDocument(form);
    const response = await linkDocumentToReservation(reservationID, created.id);
    if (!response.ok) throw new Error((await response.json()).error || 'Document opgeslagen, maar koppelen mislukt');
    form.reset();
    status.textContent = 'Document geupload en gekoppeld.';
    if (selectedReservationState?.id === reservationID) await loadReservationDetail(reservationID, 'documents');
  } catch (error) {
    status.textContent = error.message || 'Uploaden mislukt';
  }
}

async function linkExistingReservationDocument(event) {
  event.preventDefault();
  if (!selectedReservationState) return;
  const reservationID = selectedReservationState.id;
  const documentID = Number(event.currentTarget.elements.documentId.value);
  if (!documentID) return;
  const response = await linkDocumentToReservation(reservationID, documentID);
  if (!response.ok) return window.alert((await response.json()).error || 'Document koppelen mislukt');
  await loadReservationDetail(reservationID, 'documents');
}

async function unlinkReservationDocument(documentID) {
  if (!selectedReservationState) return;
  const reservationID = selectedReservationState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/reservations/${reservationID}/documents/${documentID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Document ontkoppelen mislukt');
  await loadReservationDetail(reservationID, 'documents');
}

function renderLiabilityDocuments(documents) {
  if (!documents.length) return '<p>Nog geen documenten gekoppeld.</p>';
  return `<ul>${documents.map((item) => `<li><div><a href="${apiBaseUrl}/api/v1/documents/${item.id}/content" target="_blank" rel="noopener">${escapeHtml(item.originalFilename)}</a><small>${escapeHtml(item.documentDate || item.mimeType)} &middot; ${formatFileSize(item.sizeBytes)}</small>${renderDocumentTranslation(item)}</div><button type="button" class="icon-button danger" data-liability-action="unlink-document" data-document-id="${item.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
}

function renderLiabilityDocumentOptions(allDocuments, linkedDocuments) {
  const linkedIDs = new Set(linkedDocuments.map((item) => item.id));
  const available = allDocuments.filter((item) => !linkedIDs.has(item.id));
  const form = document.querySelector('#liability-document-link-form');
  form.elements.documentId.innerHTML = available.length ? `<option value="">Kies een document</option>${available.map((item) => `<option value="${item.id}">${escapeHtml(item.originalFilename)}</option>`).join('')}` : '<option value="">Geen andere documenten beschikbaar</option>';
  form.elements.documentId.disabled = !available.length;
  form.querySelector('button').disabled = !available.length;
}

function linkDocumentToLiability(liabilityID, documentID) {
  return fetch(`${apiBaseUrl}/api/v1/liabilities/${liabilityID}/documents`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ documentId: documentID }) });
}

async function uploadLiabilityDocument(event) {
  event.preventDefault();
  if (!selectedLiabilityState) return;
  const form = event.currentTarget;
  const liabilityID = selectedLiabilityState.id;
  const status = document.querySelector('#liability-document-status');
  status.textContent = 'Document uploaden...';
  try {
    const created = await createDocument(form);
    const response = await linkDocumentToLiability(liabilityID, created.id);
    if (!response.ok) throw new Error((await response.json()).error || 'Document opgeslagen, maar koppelen mislukt');
    form.reset();
    status.textContent = 'Document geupload en gekoppeld.';
    if (selectedLiabilityState?.id === liabilityID) await loadLiabilityDetail(liabilityID, 'documents');
  } catch (error) {
    status.textContent = error.message || 'Uploaden mislukt';
  }
}

async function linkExistingLiabilityDocument(event) {
  event.preventDefault();
  if (!selectedLiabilityState) return;
  const liabilityID = selectedLiabilityState.id;
  const documentID = Number(event.currentTarget.elements.documentId.value);
  if (!documentID) return;
  const response = await linkDocumentToLiability(liabilityID, documentID);
  if (!response.ok) return window.alert((await response.json()).error || 'Document koppelen mislukt');
  await loadLiabilityDetail(liabilityID, 'documents');
}

async function unlinkLiabilityDocument(documentID) {
  if (!selectedLiabilityState) return;
  const liabilityID = selectedLiabilityState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/liabilities/${liabilityID}/documents/${documentID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Document ontkoppelen mislukt');
  await loadLiabilityDetail(liabilityID, 'documents');
}

async function loadLiabilityTransactionCandidates() {
  if (!selectedLiabilityState) return;
  const liabilityID = selectedLiabilityState.id;
  const form = document.querySelector('#liability-transaction-link-form');
  const params = new URLSearchParams({ limit: '100' });
  if (form.elements.search.value.trim()) params.set('search', form.elements.search.value.trim());
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/transactions?${params}`);
    const transactions = await response.json();
    if (!response.ok) throw new Error(transactions.error || 'Transacties zoeken mislukt');
    if (selectedLiabilityState?.id !== liabilityID) return;
    form.elements.transactionId.innerHTML = transactions.length ? `<option value="">Kies een transactie</option>${transactions.map((item) => `<option value="${item.id}">${escapeHtml(item.bookedOn)} · ${escapeHtml(item.description)} · ${formatAmount(item.amountCents)}</option>`).join('')}` : '<option value="">Geen transacties gevonden</option>';
    form.elements.transactionId.disabled = !transactions.length;
  } catch (error) {
    if (selectedLiabilityState?.id === liabilityID) window.alert(error.message);
  }
}

async function linkLiabilityTransaction(event) {
  event.preventDefault();
  if (!selectedLiabilityState) return;
  const liabilityID = selectedLiabilityState.id;
  const transactionID = Number(event.currentTarget.elements.transactionId.value);
  if (!transactionID) return;
  const response = await fetch(`${apiBaseUrl}/api/v1/liabilities/${liabilityID}/transactions`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ transactionId: transactionID }) });
  if (!response.ok) return window.alert((await response.json()).error || 'Transactie koppelen mislukt');
  await loadLiabilityDetail(liabilityID, 'transactions');
}

function renderLiabilityTransactions(transactions) {
  if (!transactions.length) return '<p>Nog geen transacties gekoppeld.</p>';
  return `<ul>${transactions.map((item) => `<li><div><strong>${escapeHtml(item.description)}</strong><small>${escapeHtml(item.bookedOn)} · ${escapeHtml(item.counterparty || '')}</small></div><span class="amount">${formatAmount(item.amountCents)}</span><button type="button" class="icon-button danger" data-liability-action="unlink-transaction" data-transaction-id="${item.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
}

async function unlinkLiabilityTransaction(transactionID) {
  if (!selectedLiabilityState) return;
  const liabilityID = selectedLiabilityState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/liabilities/${liabilityID}/transactions/${transactionID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Transactie ontkoppelen mislukt');
  await loadLiabilityDetail(liabilityID, 'transactions');
}

function caseStatusOptions(selected = '') {
  return [['planned', 'Gepland'], ['active', 'Actief'], ['completed', 'Afgerond'], ['archived', 'Gearchiveerd']]
    .map(([value, label]) => `<option value="${value}" ${value === selected ? 'selected' : ''}>${label}</option>`).join('');
}

function caseStatusLabel(value) {
  return { planned: 'Gepland', active: 'Actief', completed: 'Afgerond', archived: 'Gearchiveerd' }[value] || value;
}

async function loadCases() {
  const requestID = ++caseListRequestID;
  const form = document.querySelector('#case-filter-form');
  const params = new URLSearchParams();
  if (form.elements.caseType.value.trim()) params.set('type', form.elements.caseType.value.trim());
  if (form.elements.status.value) params.set('status', form.elements.status.value);
  const list = document.querySelector('#case-list');
  list.innerHTML = '<p>Contracten laden...</p>';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/cases?${params}`);
    const cases = await response.json();
    if (!response.ok) throw new Error(cases.error || 'Contracten laden mislukt');
    if (requestID !== caseListRequestID) return;
    casesState = cases;
    casesLoaded = true;
    list.innerHTML = renderCaseList(cases);
  } catch (error) {
    if (requestID === caseListRequestID) list.innerHTML = `<p class="error">${escapeHtml(error.message)}</p>`;
  }
}

function renderCaseList(cases) {
  if (!cases.length) return '<p>Geen contracten gevonden.</p>';
  return `<ul>${cases.map((item) => `<li><button type="button" class="asset-list-button case-list-button ${selectedCaseState?.id === item.id ? 'active' : ''}" data-case-id="${item.id}"><span><strong>${escapeHtml(item.title)}</strong><small>${escapeHtml(caseListTypeLabel(item))} &middot; ${escapeHtml(caseStatusLabel(item.status))}</small></span>${item.startDate ? `<time datetime="${escapeHtml(item.startDate)}">${escapeHtml(item.startDate)}</time>` : ''}</button></li>`).join('')}</ul>`;
}

function caseListTypeLabel(item) {
  if (item.mainCategoryId) return 'Hoofdcategorie';
  if (item.caseType === 'contract') return 'Contract';
  return item.caseType;
}

async function handleCaseAction(event) {
  const selectButton = event.target.closest('[data-case-id]');
  if (selectButton) {
    await loadCaseDetail(Number(selectButton.dataset.caseId));
    return;
  }
  const button = event.target.closest('[data-case-action], [data-case-detail-target]');
  if (!button) return;
  if (button.dataset.caseDetailTarget) {
    switchCaseDetailSection(button.dataset.caseDetailTarget);
    return;
  }
  switch (button.dataset.caseAction) {
  case 'new': showCaseForm(); break;
  case 'edit': showCaseForm(selectedCaseState); break;
  case 'cancel-form': hideCaseForm(); break;
  case 'delete': await deleteSelectedCase(); break;
  case 'unlink-document': await unlinkCaseDocument(Number(button.dataset.documentId)); break;
  case 'search-transactions': await loadCaseTransactionCandidates(); break;
  case 'unlink-transaction': await unlinkCaseTransaction(Number(button.dataset.transactionId)); break;
  case 'delete-relation': await deleteCaseRelation(Number(button.dataset.relationId)); break;
  }
}

function showCaseForm(item = null) {
  caseDetailRequestID += 1;
  const form = document.querySelector('#case-form');
  form.reset();
  form.elements.id.value = item?.id || '';
  form.elements.title.value = item?.title || '';
  form.elements.caseType.value = item?.caseType || '';
  form.elements.description.value = item?.description || '';
  form.elements.startDate.value = item?.startDate || '';
  form.elements.endDate.value = item?.endDate || '';
  form.elements.termMonths.value = item?.termMonths || '';
  form.elements.status.value = item?.status || 'active';
  const isContract = item?.caseType === 'contract';
  document.querySelector('#case-form-title').textContent = item?.id ? (isContract ? 'Contract wijzigen' : 'Object wijzigen') : 'Nieuw contract';
  document.querySelector('#case-form-panel').classList.remove('hidden');
  document.querySelector('#case-detail').classList.add('hidden');
  document.querySelector('#case-empty-state').classList.add('hidden');
}

function hideCaseForm() {
  document.querySelector('#case-form-panel').classList.add('hidden');
  document.querySelector(selectedCaseState ? '#case-detail' : '#case-empty-state').classList.remove('hidden');
}

async function saveCase(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const id = Number(form.elements.id.value);
  const input = {
    title: form.elements.title.value,
    caseType: form.elements.caseType.value,
    description: form.elements.description.value,
    startDate: form.elements.startDate.value,
    endDate: form.elements.endDate.value,
    termMonths: Number(form.elements.termMonths.value),
    status: form.elements.status.value,
  };
  try {
    const response = await fetch(id ? `${apiBaseUrl}/api/v1/cases/${id}` : `${apiBaseUrl}/api/v1/cases`, { method: id ? 'PATCH' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const item = await response.json();
    if (!response.ok) throw new Error(item.error || 'Contract opslaan mislukt');
    await loadCases();
    await loadCaseDetail(item.id);
  } catch (error) {
    window.alert(error.message);
  }
}

async function loadCaseDetail(id, targetSection = 'overview') {
  const requestID = ++caseDetailRequestID;
  try {
    const [caseResponse, documentsResponse, transactionsResponse, relationsResponse, allDocumentsResponse, assetsResponse, liabilitiesResponse, eventsResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/cases/${id}`), fetch(`${apiBaseUrl}/api/v1/cases/${id}/documents`),
      fetch(`${apiBaseUrl}/api/v1/cases/${id}/transactions`), fetch(`${apiBaseUrl}/api/v1/cases/${id}/relations`),
      fetch(`${apiBaseUrl}/api/v1/documents`), fetch(`${apiBaseUrl}/api/v1/assets`), fetch(`${apiBaseUrl}/api/v1/liabilities`),
      fetch(`${apiBaseUrl}/api/v1/cases/${id}/events`),
    ]);
    const [item, documentsResult, transactions, relations, allDocumentsResult, assets, liabilities, eventsResult] = await Promise.all([caseResponse.json(), documentsResponse.json(), transactionsResponse.json(), relationsResponse.json(), allDocumentsResponse.json(), assetsResponse.json(), liabilitiesResponse.json(), eventsResponse.json()]);
    const documentsUnavailable = documentsResponse.status === 501 || allDocumentsResponse.status === 501;
    if (!caseResponse.ok || (!documentsResponse.ok && documentsResponse.status !== 501) || !transactionsResponse.ok || !relationsResponse.ok || (!allDocumentsResponse.ok && allDocumentsResponse.status !== 501) || !assetsResponse.ok || !liabilitiesResponse.ok || !eventsResponse.ok) {
      throw new Error(item.error || documentsResult.error || transactions.error || relations.error || allDocumentsResult.error || assets.error || liabilities.error || eventsResult?.error || 'Contract laden mislukt');
    }
    const events = validateDomainEvents(eventsResult);
    if (requestID !== caseDetailRequestID) return;
    const documents = documentsResponse.status === 501 ? [] : documentsResult;
    const allDocuments = allDocumentsResponse.status === 501 ? [] : allDocumentsResult;
    selectedCaseState = item;
    caseDocumentsState = documents;
    caseTransactionsState = transactions;
    caseRelationsState = relations;
    caseAssetsState = assets;
    caseLiabilitiesState = liabilities;
    document.querySelector('#case-detail-title').textContent = item.title;
    document.querySelectorAll('#case-detail [data-case-action="edit"], #case-detail [data-case-action="delete"]').forEach((button) => button.classList.toggle('hidden', Boolean(item.mainCategoryId)));
    const categorySummary = document.querySelector('#case-category-summary');
    categorySummary.classList.toggle('hidden', !item.mainCategoryId);
    categorySummary.innerHTML = item.mainCategoryId ? renderCaseCategorySummary(transactions) : '';
    document.querySelector('#case-overview').innerHTML = renderCaseOverview(item);
    document.querySelector('#case-document-list').innerHTML = renderCaseDocuments(documents);
    document.querySelector('#case-document-status').textContent = documentsUnavailable ? 'Documentopslag is momenteel niet beschikbaar.' : '';
    renderCaseDocumentOptions(allDocuments, documents, documentsUnavailable);
    document.querySelector('#case-transaction-list').innerHTML = renderCaseTransactions(transactions);
    document.querySelector('#case-relations').innerHTML = renderCaseRelations(relations, item.id);
    document.querySelector('#case-timeline').innerHTML = renderDomainTimeline(events);
    document.querySelector('#case-detail').classList.remove('hidden');
    document.querySelector('#case-empty-state').classList.add('hidden');
    document.querySelector('#case-form-panel').classList.add('hidden');
    document.querySelector('#case-list').innerHTML = renderCaseList(casesState);
    switchCaseDetailSection(targetSection);
    updateCaseRelationForm();
    await loadCaseTransactionCandidates();
  } catch (error) {
    if (requestID === caseDetailRequestID) window.alert(error.message);
  }
}

function renderCaseCategorySummary(transactions) {
  const total = transactions.reduce((sum, transaction) => sum + transaction.amountCents, 0);
  return `<span><small>Aantal posten</small><strong>${transactions.length}</strong></span><span><small>Totale som</small><strong>${formatAmount(total)}</strong></span>`;
}

function renderCaseOverview(item) {
  return `<dl class="asset-overview-grid"><div><dt>Type</dt><dd>${escapeHtml(item.mainCategoryId ? 'Hoofdcategorie' : item.caseType)}</dd></div><div><dt>Status</dt><dd>${escapeHtml(caseStatusLabel(item.status))}</dd></div><div><dt>Startdatum</dt><dd>${escapeHtml(item.startDate || 'Niet vastgelegd')}</dd></div><div><dt>Einddatum</dt><dd>${escapeHtml(item.endDate || 'Niet vastgelegd')}</dd></div><div><dt>Looptijd</dt><dd>${escapeHtml(item.termMonths ? `${item.termMonths} maanden` : 'Niet vastgelegd')}</dd></div></dl><p class="asset-description">${escapeHtml(item.mainCategoryId ? 'Dit object wordt automatisch beheerd vanuit de hoofdcategorie.' : item.description || 'Geen omschrijving.')}</p>`;
}

function switchCaseDetailSection(target) {
  document.querySelectorAll('[data-case-detail-target]').forEach((button) => button.classList.toggle('active', button.dataset.caseDetailTarget === target));
  document.querySelectorAll('[data-case-detail-section]').forEach((section) => section.classList.toggle('active', section.dataset.caseDetailSection === target));
}

async function deleteSelectedCase() {
  if (!selectedCaseState || !window.confirm(`Contract "${selectedCaseState.title}" verwijderen?`)) return;
  const caseID = selectedCaseState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/cases/${caseID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Contract verwijderen mislukt');
  if (selectedCaseState?.id === caseID) {
    selectedCaseState = null;
    caseDetailRequestID += 1;
    document.querySelector('#case-detail').classList.add('hidden');
    document.querySelector('#case-empty-state').classList.remove('hidden');
  }
  await loadCases();
}

function renderCaseDocuments(documents) {
  if (!documents.length) return '<p>Nog geen documenten gekoppeld.</p>';
  return `<ul>${documents.map((item) => `<li><div><a href="${apiBaseUrl}/api/v1/documents/${item.id}/content" target="_blank" rel="noopener">${escapeHtml(item.originalFilename)}</a><small>${escapeHtml(item.documentDate || item.mimeType || '')}${item.sizeBytes == null ? '' : ` &middot; ${formatFileSize(item.sizeBytes)}`}</small>${item.description ? `<span>${escapeHtml(item.description)}</span>` : ''}${renderDocumentTranslation(item)}</div><button type="button" class="icon-button danger" data-case-action="unlink-document" data-document-id="${item.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
}

function renderCaseDocumentOptions(allDocuments, linkedDocuments, unavailable = false) {
  const linkedIDs = new Set(linkedDocuments.map((item) => item.id));
  const available = allDocuments.filter((item) => !linkedIDs.has(item.id));
  const form = document.querySelector('#case-document-link-form');
  form.elements.documentId.innerHTML = available.length ? `<option value="">Kies een document</option>${available.map((item) => `<option value="${item.id}">${escapeHtml(item.originalFilename)}</option>`).join('')}` : `<option value="">${unavailable ? 'Documentopslag niet beschikbaar' : 'Geen andere documenten beschikbaar'}</option>`;
  form.elements.documentId.disabled = !available.length;
  form.querySelector('button').disabled = !available.length;
  document.querySelector('#case-document-upload-form button').disabled = unavailable;
}

async function uploadCaseDocument(event) {
  event.preventDefault();
  if (!selectedCaseState) return;
  const caseID = selectedCaseState.id;
  const form = event.currentTarget;
  const status = document.querySelector('#case-document-status');
  status.textContent = 'Document uploaden...';
  try {
    const created = await createDocument(form);
    const response = await linkDocumentToCase(caseID, created.id);
    if (!response.ok) throw new Error((await response.json()).error || 'Document opgeslagen, maar koppelen mislukt');
    form.reset();
    if (selectedCaseState?.id === caseID) await loadCaseDetail(caseID, 'documents');
  } catch (error) {
    if (selectedCaseState?.id === caseID) status.textContent = error.message || 'Uploaden mislukt';
  }
}

function linkDocumentToCase(caseID, documentID) {
  return fetch(`${apiBaseUrl}/api/v1/cases/${caseID}/documents`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ documentId: documentID }) });
}

async function linkExistingCaseDocument(event) {
  event.preventDefault();
  if (!selectedCaseState) return;
  const caseID = selectedCaseState.id;
  const documentID = Number(event.currentTarget.elements.documentId.value);
  if (!documentID) return;
  const response = await linkDocumentToCase(caseID, documentID);
  if (!response.ok) return window.alert((await response.json()).error || 'Document koppelen mislukt');
  if (selectedCaseState?.id === caseID) await loadCaseDetail(caseID, 'documents');
}

async function unlinkCaseDocument(documentID) {
  if (!selectedCaseState) return;
  const caseID = selectedCaseState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/cases/${caseID}/documents/${documentID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Document ontkoppelen mislukt');
  if (selectedCaseState?.id === caseID) await loadCaseDetail(caseID, 'documents');
}

async function loadCaseTransactionCandidates() {
  if (!selectedCaseState) return;
  const caseID = selectedCaseState.id;
  const requestID = ++caseTransactionRequestID;
  const form = document.querySelector('#case-transaction-link-form');
  const params = new URLSearchParams({ limit: '100' });
  if (form.elements.search.value.trim()) params.set('search', form.elements.search.value.trim());
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/transactions?${params}`);
    const transactions = await response.json();
    if (!response.ok) throw new Error(transactions.error || 'Transacties zoeken mislukt');
    if (selectedCaseState?.id !== caseID || requestID !== caseTransactionRequestID) return;
    const linkedIDs = new Set(caseTransactionsState.map((item) => item.id));
    caseTransactionCandidatesState = transactions.filter((item) => !linkedIDs.has(item.id));
    form.elements.transactionId.innerHTML = caseTransactionCandidatesState.length ? `<option value="">Kies een transactie</option>${caseTransactionCandidatesState.map((item) => `<option value="${item.id}">${escapeHtml(item.bookedOn)} &middot; ${escapeHtml(item.description)} &middot; ${formatAmount(item.amountCents)}</option>`).join('')}` : '<option value="">Geen transacties gevonden</option>';
    form.elements.transactionId.disabled = !caseTransactionCandidatesState.length;
  } catch (error) {
    if (selectedCaseState?.id === caseID && requestID === caseTransactionRequestID) window.alert(error.message);
  }
}

async function linkCaseTransaction(event) {
  event.preventDefault();
  if (!selectedCaseState) return;
  const caseID = selectedCaseState.id;
  const transactionID = Number(event.currentTarget.elements.transactionId.value);
  if (!transactionID || !caseTransactionCandidatesState.some((item) => item.id === transactionID)) return;
  const response = await fetch(`${apiBaseUrl}/api/v1/cases/${caseID}/transactions`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ transactionId: transactionID }) });
  if (!response.ok) return window.alert((await response.json()).error || 'Transactie koppelen mislukt');
  if (selectedCaseState?.id === caseID) await loadCaseDetail(caseID, 'transactions');
}

function renderCaseTransactions(transactions) {
  if (!transactions.length) return '<p>Nog geen transacties gekoppeld.</p>';
  return `<ul>${transactions.map((item) => `<li><div><strong>${escapeHtml(item.description)}</strong><small>${escapeHtml(item.bookedOn)}${item.counterparty ? ` &middot; ${escapeHtml(item.counterparty)}` : ''}</small></div><span class="amount">${formatAmount(item.amountCents)}</span><button type="button" class="icon-button danger" data-case-action="unlink-transaction" data-transaction-id="${item.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
}

async function unlinkCaseTransaction(transactionID) {
  if (!selectedCaseState) return;
  const caseID = selectedCaseState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/cases/${caseID}/transactions/${transactionID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Transactie ontkoppelen mislukt');
  if (selectedCaseState?.id === caseID) await loadCaseDetail(caseID, 'transactions');
}

function updateCaseRelationForm() {
  const form = document.querySelector('#case-relation-form');
  const items = form.elements.targetType.value === 'asset' ? caseAssetsState : caseLiabilitiesState;
  form.elements.targetId.innerHTML = items.length ? `<option value="">Kies een object</option>${items.map((item) => `<option value="${item.id}">${escapeHtml(item.name)}</option>`).join('')}` : '<option value="">Geen objecten beschikbaar</option>';
  form.elements.targetId.disabled = !items.length;
}

function caseRelationTarget(relation, caseID) {
  const caseIsSource = (relation.sourceType === 'case' || relation.sourceType === 'owner') && relation.sourceId === caseID;
  return {
    type: caseIsSource ? relation.targetType : relation.sourceType,
    label: caseIsSource ? relation.targetLabel : relation.sourceLabel,
  };
}

function renderCaseRelations(relations, caseID) {
  if (!relations.length) return '<p>Nog geen gerelateerde objecten vastgelegd.</p>';
  return `<ul>${relations.map((relation) => {
    const related = caseRelationTarget(relation, caseID);
    const linkedItem = related.type === 'document' || related.type === 'transaction';
    const period = relation.validFrom || relation.validTo ? `<small>${escapeHtml(relation.validFrom || '...')} t/m ${escapeHtml(relation.validTo || '...')}</small>` : '';
    return `<li><div><strong>${escapeHtml(related.label || `Object ${relation.targetId || relation.sourceId}`)}</strong><small>${escapeHtml(entityTypeLabel(related.type))} &middot; ${linkedItem ? 'Gekoppeld object' : escapeHtml(relationTypeLabel(relation.relationType))}</small>${period}${relation.notes ? `<p>${escapeHtml(relation.notes)}</p>` : ''}</div><button type="button" class="icon-button danger" data-case-action="delete-relation" data-relation-id="${relation.id}" aria-label="Verwijder relatie">&#10005;</button></li>`;
  }).join('')}</ul>`;
}

async function saveCaseRelation(event) {
  event.preventDefault();
  if (!selectedCaseState) return;
  const form = event.currentTarget;
  const caseID = selectedCaseState.id;
  const targetID = Number(form.elements.targetId.value);
  if (!targetID) return;
  const input = { targetType: form.elements.targetType.value, targetId: targetID, relationType: 'relates_to', validFrom: form.elements.validFrom.value, validTo: form.elements.validTo.value, notes: form.elements.notes.value };
  const response = await fetch(`${apiBaseUrl}/api/v1/cases/${caseID}/relations`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
  if (!response.ok) return window.alert((await response.json()).error || 'Relatie toevoegen mislukt');
  form.elements.validFrom.value = '';
  form.elements.validTo.value = '';
  form.elements.notes.value = '';
  if (selectedCaseState?.id === caseID) await loadCaseDetail(caseID, 'relations');
}

async function deleteCaseRelation(relationID) {
  if (!selectedCaseState || !window.confirm('Deze relatie verwijderen?')) return;
  const caseID = selectedCaseState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/cases/${caseID}/relations/${relationID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Relatie verwijderen mislukt');
  if (selectedCaseState?.id === caseID) await loadCaseDetail(caseID, 'relations');
}

function validateDomainEvents(events) {
  const validOptionalString = (value) => value == null || typeof value === 'string';
  const validOptionalID = (value) => value == null || Number.isInteger(value);
  if (!Array.isArray(events) || events.some((event) => !event || typeof event !== 'object'
    || !Number.isInteger(event.id) || typeof event.entityType !== 'string' || !Number.isInteger(event.entityId)
    || typeof event.eventType !== 'string' || typeof event.eventDate !== 'string' || typeof event.description !== 'string'
    || typeof event.createdAt !== 'string' || !validOptionalString(event.relatedEntityType)
    || !validOptionalID(event.relatedEntityId) || !validOptionalString(event.relatedEntityLabel))) {
    throw new Error('Tijdlijn bevat ongeldige gegevens');
  }
  return events;
}

function renderDomainTimeline(events) {
  if (!events.length) return '<div class="empty-state"><strong>Nog geen gebeurtenissen</strong><p>Gebeurtenissen verschijnen hier zodra ze zijn vastgelegd.</p></div>';
  const sortedEvents = [...events].sort((left, right) => String(right.eventDate).localeCompare(String(left.eventDate)) || right.id - left.id);
  return `<ol>${sortedEvents.map((event) => `<li><time datetime="${escapeHtml(event.eventDate)}">${escapeHtml(formatTimelineDate(event.eventDate))}</time><div><strong>${escapeHtml(domainEventTypeLabel(event.eventType))}</strong><p>${escapeHtml(event.description)}</p>${renderDomainEventRelatedObject(event)}</div></li>`).join('')}</ol>`;
}

function domainEventTypeLabel(value) {
  const labels = {
    asset_created: 'Bezitting aangemaakt', asset_updated: 'Bezitting gewijzigd',
    valuation_added: 'Waardering toegevoegd', valuation_updated: 'Waardering gewijzigd', valuation_removed: 'Waardering verwijderd',
    document_linked: 'Document gekoppeld', document_unlinked: 'Document ontkoppeld',
    transaction_linked: 'Transactie gekoppeld', transaction_unlinked: 'Transactie ontkoppeld',
    relation_linked: 'Relatie gekoppeld', relation_unlinked: 'Relatie ontkoppeld',
    liability_created: 'Schuld aangemaakt', liability_updated: 'Schuld gewijzigd', liability_repaid: 'Schuld afgelost',
    case_created: 'Contract aangemaakt', case_updated: 'Contract gewijzigd', case_completed: 'Contract afgerond',
  };
  return Object.hasOwn(labels, value) ? labels[value] : value;
}

function renderDomainEventRelatedObject(event) {
  const type = event.relatedEntityType;
  const id = Number(event.relatedEntityId);
  const hasNumericID = Number.isSafeInteger(id) && id > 0;
  if (!type) return '';
  if (type === 'document' && hasNumericID) {
    return `<a href="${apiBaseUrl}/api/v1/documents/${id}/content" target="_blank" rel="noopener">${escapeHtml(event.relatedEntityLabel || 'Document openen')}</a>`;
  }
  const label = event.relatedEntityLabel || (hasNumericID ? `${entityTypeLabel(type)} ${id}` : entityTypeLabel(type));
  if (hasNumericID && ['asset', 'liability', 'case'].includes(type)) {
    return `<button type="button" class="domain-timeline-link" data-related-domain-type="${type}" data-related-domain-id="${id}">${escapeHtml(label)}</button>`;
  }
  return `<span class="domain-timeline-related">${escapeHtml(label)}</span>`;
}

function formatTimelineDate(value) {
  const date = new Date(`${String(value).slice(0, 10)}T00:00:00`);
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('nl-NL', { day: 'numeric', month: 'short', year: 'numeric' }).format(date);
}

async function loadHiddenTransactions() {
  const result = document.querySelector('#hidden-transactions-result');
  result.innerHTML = '<p>Verborgen transacties laden...</p>';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/transactions?hidden=true&limit=100`);
    const transactions = await response.json();
    if (!response.ok) {
      throw new Error(transactions.error || 'Verborgen transacties laden mislukt');
    }
    updateHiddenTransactionSummary(transactions);
    result.innerHTML = renderHiddenTransactionTable(transactions);
  } catch (error) {
    updateHiddenTransactionSummary([]);
    result.innerHTML = `<p class="error">${error.message}</p>`;
  }
}

function renderHiddenTransactionTable(transactions) {
  if (!transactions.length) {
    return '<p>Geen verborgen transacties.</p>';
  }
  const rows = transactions.map((transaction) => `
    <tr data-transaction-id="${transaction.id}">
      <td>${escapeHtml(transaction.bookedOn)}</td>
      <td><span class="transaction-description">${escapeHtml(transaction.description)}</span>${renderDocumentControl(transaction)}</td>
      <td>${escapeHtml(transaction.counterparty || '')}</td>
      <td class="amount">${formatAmount(transaction.amountCents)}</td>
      <td><button type="button" class="icon-button" data-action="unhide-transaction" aria-label="Maak transactie zichtbaar">&#8617;</button></td>
    </tr>
  `).join('');

  return `
    <table class="hidden-transaction-table">
      <thead><tr><th>Datum</th><th>Omschrijving</th><th>Tegenpartij</th><th>Bedrag</th><th>Zichtbaar</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function updateHiddenTransactionSummary(transactions) {
  const count = document.querySelector('#hidden-transaction-count');
  const total = document.querySelector('#hidden-transaction-total');
  if (!count || !total) {
    return;
  }
  count.textContent = String(transactions.length);
  total.textContent = formatAmount(transactions.reduce((sum, transaction) => sum + transaction.amountCents, 0));
}

function updateVisibleCategorizedRows(sourceRow, categoryID, applyToSameCounterparty) {
  const category = categoryName(categoryID);
  const rows = applyToSameCounterparty
    ? Array.from(document.querySelectorAll('tr[data-counterparty]')).filter((row) => row.dataset.counterparty === sourceRow.dataset.counterparty && !row.dataset.categoryId)
    : [sourceRow];

  rows.forEach((row) => {
    if (accountTransactionsUncategorizedOnly) {
      row.remove();
      return;
    }
    row.querySelector('[data-category-label]').textContent = category;
    row.querySelector('[data-category-select]').value = String(categoryID);
    row.dataset.categoryId = String(categoryID);
  });
}

function showCategorizeStatus(row, message) {
  const cell = row.querySelector('.categorize-cell');
  if (!cell) {
    return;
  }
  cell.insertAdjacentHTML('beforeend', `<small class="save-status">${escapeHtml(message)}</small>`);
}

function renderMainCategoryList() {
  return mainCategoriesState.map((category) => `
    <li data-main-category-id="${category.id}">
      <span>${escapeHtml(category.name)}</span>
      <span class="row-actions">
        <button type="button" class="icon-button" data-action="edit-main" data-id="${category.id}" aria-label="Wijzig ${escapeHtml(category.name)}">&#9998;</button>
        <button type="button" class="icon-button danger" data-action="delete-main" data-id="${category.id}" aria-label="Verwijder ${escapeHtml(category.name)}">&#10005;</button>
      </span>
    </li>
  `).join('');
}

function renderSubcategoryEditor(mainCategoryID) {
  const mainCategory = mainCategoriesState.find((category) => category.id === mainCategoryID);
  if (!mainCategory) {
    return;
  }

  const subcategories = categoriesState.filter((category) => category.mainCategoryId === mainCategoryID);
  const rows = subcategories.length
    ? subcategories.map((category) => `
      <li>
        <span>${escapeHtml(category.name)}</span>
        <small>${escapeHtml(category.kind)} / ${escapeHtml(category.purpose)}</small>
        <span class="row-actions">
          <button type="button" class="icon-button" data-action="edit-subcategory" data-id="${category.id}" aria-label="Wijzig ${escapeHtml(category.name)}">&#9998;</button>
          <button type="button" class="icon-button danger" data-action="delete-subcategory" data-id="${category.id}" aria-label="Verwijder ${escapeHtml(category.name)}">&#10005;</button>
        </span>
      </li>
    `).join('')
    : '<li><span>Nog geen subcategorieen</span></li>';

  document.querySelector('#subcategory-editor').innerHTML = `
    <div class="subcategory-heading">
      <div>
        <p class="eyebrow">Subcategorieen</p>
        <h3>${escapeHtml(mainCategory.name)}</h3>
      </div>
      <button type="button" class="secondary-button" data-action="rename-main" data-id="${mainCategory.id}">Naam wijzigen</button>
    </div>
    <form id="subcategory-form" class="subcategory-form" data-main-category-id="${mainCategory.id}">
      <input name="id" type="hidden" />
      <label>
        Nieuwe subcategorie
        <input name="name" type="text" placeholder="Bijvoorbeeld Energie" required />
      </label>
      <label>
        Soort
        <select name="kind">
          <option value="expense">Uitgave</option>
          <option value="income">Inkomst</option>
          <option value="transfer">Overboeking</option>
        </select>
      </label>
      <label>
        Doel
        <select name="purpose">
          <option value="daily">Dagelijks</option>
          <option value="reservation">Reservering</option>
          <option value="investment">Investering</option>
        </select>
      </label>
      <div class="form-actions">
        <button type="submit" id="subcategory-submit">Subcategorie toevoegen</button>
        <button type="button" class="secondary-button hidden" id="subcategory-cancel" data-action="cancel-subcategory-edit">Annuleren</button>
      </div>
    </form>
    <ul class="subcategory-list">${rows}</ul>
  `;

  document.querySelector('#subcategory-form').addEventListener('submit', saveSubcategory);
}

function switchConfigSection(event) {
  const target = event.currentTarget.dataset.configTarget;
  document.querySelectorAll('.config-tab').forEach((button) => {
    const active = button.dataset.configTarget === target;
    button.classList.toggle('active', active);
    button.setAttribute('aria-selected', String(active));
  });
  document.querySelectorAll('.config-section').forEach((section) => {
    section.classList.toggle('active', section.dataset.configSection === target);
  });
  if (target === 'system') {
    loadSystemConfiguration();
  }
  if (target === 'backup') {
    loadBackupConfiguration();
  }
}

function switchMainSection(event) {
  const target = event.currentTarget.dataset.mainTarget;
  activateMainSection(target);
}

function activateMainSection(target, wealthTarget = '') {
  document.querySelectorAll('.main-tab').forEach((button) => {
    button.classList.toggle('active', button.dataset.mainTarget === target);
  });
  document.querySelectorAll('.app-tab-section').forEach((section) => {
    section.classList.toggle('active', section.dataset.mainSection === target);
  });
  if (target === 'dashboard') {
    loadDashboardWealth();
  }
  if (target === 'wealth') {
    activateWealthSection(wealthTarget || document.querySelector('.wealth-tab.active')?.dataset.wealthSectionTarget || 'assets');
  }
  if (target === 'cases' && !casesLoaded) {
    loadCases();
  }
  if (target === 'configuration' && document.querySelector('.config-tab.active')?.dataset.configTarget === 'system') {
    loadSystemConfiguration();
  }
  if (target === 'configuration' && document.querySelector('.config-tab.active')?.dataset.configTarget === 'backup') {
    loadBackupConfiguration();
  }
}

function switchWealthSection(event) {
  activateWealthSection(event.currentTarget.dataset.wealthSectionTarget);
}

function activateWealthSection(target) {
  document.querySelectorAll('.wealth-tab').forEach((button) => {
    const active = button.dataset.wealthSectionTarget === target;
    button.classList.toggle('active', active);
    button.setAttribute('aria-selected', String(active));
  });
  document.querySelectorAll('.wealth-section').forEach((section) => section.classList.toggle('active', section.dataset.wealthSection === target));
  if (target === 'assets' && !assetsLoaded) loadAssets();
  if (target === 'liabilities' && !liabilitiesLoaded) loadLiabilities();
  if (target === 'reservations' && !reservationsLoaded) loadReservations();
}

async function loadSystemConfiguration(force = false) {
  if (systemConfigurationLoading || (systemConfigurationLoaded && !force)) return;
  const root = document.querySelector('#system-configuration');
  if (!root) return;
  systemConfigurationLoading = true;
  root.innerHTML = '<p class="system-config-status" role="status" aria-live="polite">Systeemconfiguratie laden...</p>';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/system/configuration`);
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || data.message || 'Systeemconfiguratie kon niet worden geladen');
    systemConfigurationLoaded = true;
    root.innerHTML = renderSystemConfiguration(data);
  } catch (error) {
    root.innerHTML = `<div class="system-config-status error" role="alert" aria-live="assertive"><p>${escapeHtml(error.message)}</p><button type="button" class="secondary-button" data-system-action="retry">Opnieuw proberen</button></div>`;
  } finally {
    systemConfigurationLoading = false;
  }
}

function systemCapability(configuration, group, value) {
  const item = configuration.capabilities?.[group]?.find((option) => option.value === value);
  return { supported: Boolean(item?.supported), label: item?.label || '', reason: item?.reason || '' };
}

function systemChoice(configuration, group, name, value, label, description) {
  const capability = systemCapability(configuration, group, value);
  const selected = configuration[name] === value;
  const status = capability.label || label;
  return `
    <label class="system-choice ${capability.supported ? '' : 'unsupported'}">
      <input type="radio" name="${name}" value="${value}" ${selected ? 'checked' : ''} ${capability.supported && configuration.editable ? '' : 'disabled'} />
      <span><strong>${escapeHtml(status)}</strong><small>${description}</small></span>
      ${capability.supported ? '' : `<em>${escapeHtml(capability.reason || 'Binnenkort beschikbaar')}</em>`}
    </label>`;
}

function secretStatus(isSet) {
  return `<span class="secret-status">${isSet ? 'Ingesteld' : 'Niet ingesteld'}</span>`;
}

function systemValue(value) {
  return escapeHtml(value ?? '');
}

function tlsOptions(values, selected) {
  return values.map(([value, label]) => `<option value="${value}" ${value === selected ? 'selected' : ''}>${label}</option>`).join('');
}

function renderSystemConfiguration(configuration, statusMessage = '') {
  const database = configuration.database || {};
  const newDatabaseContract = Boolean(database.sqlite || database.postgres || database.mysql);
  const sqlite = database.sqlite || { path: database.backend === 'sqlite' ? database.name : '' };
  const postgres = database.postgres || {
    host: database.host, port: database.port || 5432, name: database.name, user: database.user,
    tlsMode: database.sslMode, passwordSet: database.passwordSet,
  };
  const mysql = database.mysql || { host: '', port: 3306, name: '', user: '', tlsMode: 'required', passwordSet: false };
  const objects = configuration.objects || {};
  const llm = configuration.llm || {};
  const profiles = [
    ['local', 'Lokale computer', 'Voor gebruik op een enkele computer'],
    ['server', 'Eigen server', 'Voor een eigen beheerde server'],
    ['central', 'Centrale omgeving', 'Voor centraal beheerde opslag'],
  ];
  const readOnly = !configuration.editable;
  const sourceMessage = readOnly
    ? (configuration.readOnlyReason || (configuration.source === 'environment' ? 'Deze effectieve instellingen worden beheerd via omgevingsvariabelen en kunnen hier niet worden gewijzigd.' : 'Deze effectieve instellingen zijn alleen-lezen en kunnen hier niet worden gewijzigd.'))
    : 'Deze instellingen worden opgeslagen in het configuratiebestand.';
  const message = statusMessage || configuration.message || (configuration.restartRequired ? 'Herstart de applicatie om de wijzigingen toe te passen.' : '');
  const disabled = readOnly ? 'disabled' : '';
  return `
    <header class="system-config-heading">
      <div><p class="eyebrow">Opslag en gebruik</p><h2>Systeemconfiguratie</h2></div>
      <span class="config-source-badge">${configuration.source === 'environment' ? 'Omgevingsvariabelen' : 'Configuratiebestand'}</span>
    </header>
    <p class="system-config-explanation">${escapeHtml(sourceMessage)}</p>
    <p class="system-config-explanation warning">Een andere opslagkeuze migreert bestaande databasegegevens of documenten niet.</p>
    <p class="system-config-status ${configuration.restartRequired ? 'warning' : ''}" role="status" aria-live="polite">${escapeHtml(message)}</p>
    <form id="system-configuration-form" class="system-configuration-form" data-database-contract="${newDatabaseContract ? 'v2' : 'legacy'}">
      <fieldset ${disabled}>
        <legend>Gebruiksprofiel</legend>
        <div class="system-profile-choices">${profiles.map(([value, label, description]) => systemChoice(configuration, 'profiles', 'profile', value, label, description)).join('')}</div>
      </fieldset>
      <fieldset ${disabled}>
        <legend>Database</legend>
        <div class="system-backend-choices">
          ${systemChoice({ ...configuration, databaseBackend: database.backend }, 'databases', 'databaseBackend', 'postgres', 'PostgreSQL', 'Database op een server')}
          ${systemChoice({ ...configuration, databaseBackend: database.backend }, 'databases', 'databaseBackend', 'sqlite', 'SQLite', 'Lokaal databasebestand')}
          ${systemChoice({ ...configuration, databaseBackend: database.backend }, 'databases', 'databaseBackend', 'mysql', 'MySQL', 'Database op een server')}
        </div>
        <div class="system-fields ${database.backend === 'postgres' ? '' : 'hidden'}" data-database-fields="postgres">
          <label>Host <input name="postgresHost" type="text" value="${systemValue(postgres.host)}" /></label>
          <label>Poort <input name="postgresPort" type="number" min="1" max="65535" value="${systemValue(postgres.port || 5432)}" /></label>
          <label>Databasenaam <input name="postgresName" type="text" value="${systemValue(postgres.name)}" /></label>
          <label>Gebruiker <input name="postgresUser" type="text" value="${systemValue(postgres.user)}" /></label>
          <label>TLS-modus <select name="postgresTlsMode">${tlsOptions([['disable', 'Uitgeschakeld'], ['require', 'Vereist'], ['verify-ca', 'CA verifieren'], ['verify-full', 'CA en host verifieren']], postgres.tlsMode)}</select></label>
          <label class="secret-field">Wachtwoord ${secretStatus(postgres.passwordSet)}<input name="postgresPassword" type="password" value="" autocomplete="new-password" placeholder="Leeg laten om huidig wachtwoord te behouden" /></label>
        </div>
        <div class="system-fields ${database.backend === 'sqlite' ? '' : 'hidden'}" data-database-fields="sqlite"><label>Absoluut databasepad <input name="sqlitePath" type="text" value="${systemValue(sqlite.path)}" placeholder="/app/data/sqlite/hohoho.db" /></label></div>
        <div class="system-fields ${database.backend === 'mysql' ? '' : 'hidden'}" data-database-fields="mysql">
          <label>Host <input name="mysqlHost" type="text" value="${systemValue(mysql.host)}" /></label>
          <label>Poort <input name="mysqlPort" type="number" min="1" max="65535" value="${systemValue(mysql.port || 3306)}" /></label>
          <label>Databasenaam <input name="mysqlName" type="text" value="${systemValue(mysql.name)}" /></label>
          <label>Gebruiker <input name="mysqlUser" type="text" value="${systemValue(mysql.user)}" /></label>
          <label>TLS-modus <select name="mysqlTlsMode">${tlsOptions([['required', 'Vereist - geen plaintext fallback'], ['disabled', 'Uitgeschakeld - alleen vertrouwd lokaal netwerk/Compose']], mysql.tlsMode === 'disabled' ? 'disabled' : 'required')}</select></label>
          <label class="secret-field">Wachtwoord ${secretStatus(mysql.passwordSet)}<input name="mysqlPassword" type="password" value="" autocomplete="new-password" placeholder="Leeg laten om huidig wachtwoord te behouden" /></label>
          <p class="mysql-tls-note">De lokale Compose-service biedt geen TLS. Kies daar alleen 'Uitgeschakeld' omdat verkeer binnen het vertrouwde Compose-netwerk blijft.</p>
        </div>
      </fieldset>
      <fieldset ${disabled}>
        <legend>Objectopslag</legend>
        <div class="system-backend-choices">
          ${systemChoice({ ...configuration, objectsBackend: objects.backend }, 'objectStores', 'objectsBackend', 's3', 'S3', 'S3-compatibele objectopslag')}
          ${systemChoice({ ...configuration, objectsBackend: objects.backend }, 'objectStores', 'objectsBackend', 'filesystem', 'Bestandssysteem', 'Lokale bestandsopslag')}
        </div>
        <div class="system-fields ${objects.backend === 's3' ? '' : 'hidden'}" data-object-fields="s3">
          <label>Endpoint (host:poort) <input name="objectsEndpoint" type="text" value="${systemValue(objects.endpoint)}" placeholder="garage.example.nl:3900" /></label>
          <label>Bucket <input name="objectsBucket" type="text" value="${systemValue(objects.bucket)}" /></label>
          <label>Regio <input name="objectsRegion" type="text" value="${systemValue(objects.region)}" /></label>
          <label class="system-checkbox"><input name="objectsUseTLS" type="checkbox" ${objects.useTLS ? 'checked' : ''} /> TLS gebruiken</label>
          <label class="system-checkbox"><input name="objectsPathStyle" type="checkbox" ${objects.pathStyle ? 'checked' : ''} /> Path-style adressen</label>
          <label class="system-checkbox"><input name="objectsCreateBucket" type="checkbox" ${objects.createBucket ? 'checked' : ''} /> Bucket automatisch aanmaken</label>
          <label class="secret-field">Access key ${secretStatus(objects.accessKeySet)}<input name="objectsAccessKey" type="password" value="" autocomplete="new-password" placeholder="Leeg laten om huidige sleutel te behouden" /></label>
          <label class="secret-field">Secret key ${secretStatus(objects.secretKeySet)}<input name="objectsSecretKey" type="password" value="" autocomplete="new-password" placeholder="Leeg laten om huidige sleutel te behouden" /></label>
        </div>
        <div class="system-fields ${objects.backend === 'filesystem' ? '' : 'hidden'}" data-object-fields="filesystem"><label>Opslagpad <input name="objectsPath" type="text" value="${systemValue(objects.path)}" /></label></div>
      </fieldset>
      <fieldset ${disabled}>
        <legend>Lokale LLM</legend>
        <p class="system-config-explanation">PDF-documentanalyse loopt via de centrale PDF-AI-service. Transactieclassificatie gebruikt later het lokale LLM-endpoint.</p>
        <div class="system-fields">
          <label>HTTP(S)-endpoint <input name="llmEndpoint" type="url" value="${systemValue(llm.endpoint)}" placeholder="http://192.168.178.215:8080" /></label>
          <label>PDF-AI endpoint <input name="pdfAiUrl" type="url" value="${systemValue(llm.pdfAiUrl || 'http://192.168.178.215:8090')}" placeholder="http://192.168.178.215:8090" /></label>
          <label class="system-checkbox"><input name="llmDocumentMetadata" type="checkbox" ${llm.documentMetadata ? 'checked' : ''} /> PDF-documenten analyseren via PDF-AI</label>
          <label class="system-checkbox"><input name="llmTransactionClassify" type="checkbox" ${llm.transactionClassify ? 'checked' : ''} /> Banktransacties bij import classificeren</label>
        </div>
      </fieldset>
      ${readOnly ? '' : '<button type="submit" class="system-save-button">Systeemconfiguratie opslaan</button>'}
    </form>`;
}

function updateSystemConfigurationFields(event) {
  if (!event.target.matches('input[type="radio"]')) return;
  const form = event.currentTarget.querySelector('form');
  if (!form) return;
  if (event.target.name === 'databaseBackend' && event.target.value === 'sqlite') {
    form.elements.profile.value = 'local';
    form.elements.objectsBackend.value = 'filesystem';
  } else if (event.target.name === 'databaseBackend' && form.elements.profile.value === 'local') {
    form.elements.profile.value = 'server';
  } else if (event.target.name === 'profile' && event.target.value === 'local') {
    form.elements.databaseBackend.value = 'sqlite';
    form.elements.objectsBackend.value = 'filesystem';
  } else if (event.target.name === 'profile' && form.elements.databaseBackend.value === 'sqlite') {
    form.elements.databaseBackend.value = 'postgres';
  } else if (event.target.name === 'objectsBackend' && event.target.value === 's3' && form.elements.profile.value === 'local') {
    form.elements.profile.value = 'server';
    form.elements.databaseBackend.value = 'postgres';
  }
  const databaseBackend = form.elements.databaseBackend.value;
  const objectBackend = form.elements.objectsBackend.value;
  form.querySelectorAll('[data-database-fields]').forEach((section) => section.classList.toggle('hidden', section.dataset.databaseFields !== databaseBackend));
  form.querySelectorAll('[data-object-fields]').forEach((section) => section.classList.toggle('hidden', section.dataset.objectFields !== objectBackend));
}

function handleSystemConfigurationClick(event) {
  if (event.target.closest('[data-system-action="retry"]')) loadSystemConfiguration(true);
}

async function saveSystemConfiguration(event) {
  if (event.target.id !== 'system-configuration-form') return;
  event.preventDefault();
  const form = event.target;
  const status = form.previousElementSibling;
  const submit = form.querySelector('button[type="submit"]');
  const databaseBackend = form.elements.databaseBackend.value;
  const objectBackend = form.elements.objectsBackend.value;
  const input = {
    profile: form.elements.profile.value,
    database: { backend: databaseBackend },
    objects: objectBackend === 's3' ? {
      backend: objectBackend, endpoint: form.elements.objectsEndpoint.value, bucket: form.elements.objectsBucket.value,
      region: form.elements.objectsRegion.value, useTLS: form.elements.objectsUseTLS.checked,
      pathStyle: form.elements.objectsPathStyle.checked, createBucket: form.elements.objectsCreateBucket.checked,
    } : { backend: objectBackend, path: form.elements.objectsPath.value },
    llm: {
      endpoint: form.elements.llmEndpoint.value,
      pdfAiUrl: form.elements.pdfAiUrl.value,
      documentMetadata: form.elements.llmDocumentMetadata.checked,
      transactionClassify: form.elements.llmTransactionClassify.checked,
    },
  };
  if (form.dataset.databaseContract === 'v2') {
    input.database.postgres = {
      host: form.elements.postgresHost.value, port: Number(form.elements.postgresPort.value), name: form.elements.postgresName.value,
      user: form.elements.postgresUser.value, tlsMode: form.elements.postgresTlsMode.value,
    };
    if (form.elements.sqlitePath.value.startsWith('/')) input.database.sqlite = { path: form.elements.sqlitePath.value };
    if (form.elements.postgresPassword.value) input.database.postgres.password = form.elements.postgresPassword.value;
    if (form.elements.mysqlHost.value || form.elements.mysqlName.value || form.elements.mysqlUser.value || form.elements.mysqlPassword.value) {
      input.database.mysql = {
        host: form.elements.mysqlHost.value, port: Number(form.elements.mysqlPort.value), name: form.elements.mysqlName.value,
        user: form.elements.mysqlUser.value, tlsMode: form.elements.mysqlTlsMode.value,
      };
      if (form.elements.mysqlPassword.value) input.database.mysql.password = form.elements.mysqlPassword.value;
    }
  } else if (databaseBackend === 'postgres') {
    Object.assign(input.database, {
      host: form.elements.postgresHost.value, port: Number(form.elements.postgresPort.value), name: form.elements.postgresName.value,
      user: form.elements.postgresUser.value, sslMode: form.elements.postgresTlsMode.value,
    });
    if (form.elements.postgresPassword.value) input.database.password = form.elements.postgresPassword.value;
  } else {
    input.database.name = form.elements.sqlitePath.value;
  }
  if (form.elements.objectsAccessKey?.value) input.objects.accessKey = form.elements.objectsAccessKey.value;
  if (form.elements.objectsSecretKey?.value) input.objects.secretKey = form.elements.objectsSecretKey.value;
  submit.disabled = true;
  status.className = 'system-config-status';
  status.textContent = 'Systeemconfiguratie opslaan...';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/system/configuration`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || data.message || 'Systeemconfiguratie opslaan mislukt');
    document.querySelector('#system-configuration').innerHTML = renderSystemConfiguration(data, data.restartRequired ? 'Opgeslagen. Herstart de applicatie om de wijzigingen toe te passen.' : 'Systeemconfiguratie opgeslagen.');
  } catch (error) {
    status.className = 'system-config-status error';
    status.textContent = error.message;
    submit.disabled = false;
  }
}

async function loadBackupConfiguration(force = false) {
  if (backupConfigurationLoading || (backupConfigurationLoaded && !force)) return;
  const root = document.querySelector('#backup-configuration');
  if (!root) return;
  backupConfigurationLoading = true;
  root.innerHTML = '<p class="system-config-status" role="status" aria-live="polite">Backupconfiguratie laden...</p>';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/system/backup`);
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || 'Backupconfiguratie kon niet worden geladen');
    backupConfigurationLoaded = true;
    root.innerHTML = renderBackupConfiguration(data);
    scheduleBackupPoll(data);
  } catch (error) {
    root.innerHTML = `<div class="system-config-status error" role="alert"><p>${escapeHtml(error.message)}</p><button type="button" class="secondary-button" data-backup-action="retry">Opnieuw proberen</button></div>`;
  } finally {
    backupConfigurationLoading = false;
  }
}

function renderBackupConfiguration(data, message = '') {
  const state = data.running ? 'Backup wordt gemaakt' : data.validated ? 'Gereed' : data.configured ? 'Niet getest' : 'Niet geconfigureerd';
  const status = message || (data.lastError ? 'De laatste backup is mislukt.' : data.validated ? 'Doelopslag is succesvol getest.' : 'Sla de instellingen op en test de doelopslag.');
  return `
    <header class="system-config-heading"><div><p class="eyebrow">Versleuteld en extern</p><h2>Systeembackup</h2></div><span class="config-source-badge">${state}</span></header>
    <p class="system-config-explanation">Maakt een volledige backup van de database, documenten en systeemconfiguratie. Restic versleutelt de inhoud; rclone verstuurt deze naar de gekozen opslag.</p>
    <p id="backup-config-status" class="system-config-status ${data.lastError ? 'error' : ''}" role="status" aria-live="polite">${escapeHtml(status)}</p>
    <form id="backup-configuration-form" class="system-configuration-form backup-configuration-form">
      <fieldset ${data.running ? 'disabled' : ''}>
        <legend>Doelopslag</legend>
        <div class="system-fields backup-fields">
          <label>Rclone-doel <input name="repository" type="text" value="${escapeHtml(data.repository || '')}" placeholder="remote:hohoho-backups" required /></label>
          <label class="secret-field">Restic-wachtwoord ${secretStatus(data.resticPasswordSet)}<input name="resticPassword" type="password" autocomplete="new-password" placeholder="${data.resticPasswordSet ? 'Leeg laten om huidig wachtwoord te behouden' : 'Sterk herstelwachtwoord'}" ${data.resticPasswordSet ? '' : 'required'} /></label>
          <label class="backup-rclone-config">Rclone-configuratie ${secretStatus(data.rcloneConfigSet)}<textarea name="rcloneConfig" rows="12" placeholder="[remote]&#10;type = s3&#10;..." ${data.rcloneConfigSet ? '' : 'required'}></textarea><small>Gebruik de configuratie van een bestaande rclone-remote. Deze bevat meestal geheime toegangsgegevens.</small></label>
        </div>
      </fieldset>
      <div class="backup-actions">
        <button type="submit" ${data.running ? 'disabled' : ''}>Opslaan en testen</button>
        <button type="button" class="secondary-button" data-backup-action="validate" ${!data.configured || data.running ? 'disabled' : ''}>Opnieuw testen</button>
        <button type="button" data-backup-action="run" ${!data.validated || data.running ? 'disabled' : ''}>Nu backup maken</button>
      </div>
    </form>
    <section class="backup-last-run">
      <h3>Laatste uitvoering</h3>
      <dl class="asset-overview-grid">
        <div><dt>Gestart</dt><dd>${formatBackupTimestamp(data.lastStartedAt)}</dd></div>
        <div><dt>Afgerond</dt><dd>${formatBackupTimestamp(data.lastFinishedAt)}</dd></div>
        <div><dt>Laatste succes</dt><dd>${formatBackupTimestamp(data.lastSuccessAt)}</dd></div>
        <div><dt>Snapshot</dt><dd>${escapeHtml(data.lastSnapshotId || 'Nog niet beschikbaar')}</dd></div>
      </dl>
    </section>
    ${data.configured ? renderBackupRecovery(data) : ''}`;
}

function renderBackupRecovery(data) {
  const recoveryStatus = data.recoveryState === 'failed'
    ? 'Het laatste herstel is mislukt. Gebruik zo nodig het noodherstelcommando.'
    : data.recoveryState === 'succeeded'
      ? `Laatste herstel voltooid op ${formatBackupTimestamp(data.recoveryFinishedAt)}.`
      : 'Laad de beschikbare snapshots om een herstelpunt te kiezen.';
  return `
    <section id="backup-recovery" class="backup-recovery">
      <div class="backup-recovery-heading"><div><p class="eyebrow">Noodherstel</p><h3>Backup herstellen</h3></div></div>
      <div class="backup-recovery-warning" role="alert">
        <strong>Let op: dit vervangt alle huidige gegevens.</strong>
        <p>De database, documenten en systeemconfiguratie worden volledig vervangen door de gekozen snapshot. Vooraf wordt automatisch een extra veiligheidsbackup gemaakt.</p>
      </div>
      <p id="backup-recovery-status" class="system-config-status ${data.recoveryState === 'failed' ? 'error' : ''}" role="status" aria-live="polite">${escapeHtml(recoveryStatus)}</p>
      <div class="backup-recovery-controls">
        <label for="backup-recovery-snapshot">Snapshot
          <select id="backup-recovery-snapshot" disabled>
            <option value="">Snapshots nog niet geladen</option>
          </select>
        </label>
        <button type="button" class="secondary-button" data-backup-action="snapshots" ${data.running ? 'disabled' : ''}>Snapshots laden</button>
        <button type="button" class="backup-restore-button" data-backup-action="restore" disabled>Gekozen backup herstellen</button>
      </div>
    </section>`;
}

function formatBackupTimestamp(value) {
  if (!value) return 'Nog niet beschikbaar';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? escapeHtml(value) : escapeHtml(date.toLocaleString('nl-NL'));
}

function invalidateBackupConfiguration(event) {
  if (event && !event.target.closest('#backup-configuration-form')) return;
  const run = document.querySelector('[data-backup-action="run"]');
  if (run) run.disabled = true;
  const status = document.querySelector('#backup-config-status');
  if (status) {
    status.className = 'system-config-status';
    status.textContent = 'Gewijzigde instellingen eerst opslaan en testen.';
  }
}

async function saveBackupConfiguration(event) {
  if (event.target.id !== 'backup-configuration-form') return;
  event.preventDefault();
  const form = event.target;
  const input = { repository: form.elements.repository.value };
  if (form.elements.rcloneConfig.value) input.rcloneConfig = form.elements.rcloneConfig.value;
  if (form.elements.resticPassword.value) input.resticPassword = form.elements.resticPassword.value;
  await performBackupAction('Backupdoel opslaan en testen...', async () => {
    const response = await fetch(`${apiBaseUrl}/api/v1/system/backup`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(backupValidationMessage(data.error));
    return data;
  }, 'Backupconfiguratie opgeslagen; doelopslag is gereed.');
}

async function handleBackupConfigurationClick(event) {
  const action = event.target.closest('[data-backup-action]')?.dataset.backupAction;
  if (!action) return;
  if (action === 'retry') return loadBackupConfiguration(true);
  if (action === 'validate') {
    return performBackupAction('Doelopslag testen...', async () => {
      const response = await fetch(`${apiBaseUrl}/api/v1/system/backup/validate`, { method: 'POST' });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(backupValidationMessage(data.error));
      return data;
    }, 'Doelopslag is succesvol getest.');
  }
  if (action === 'run') {
    if (!window.confirm('Nu een volledige systeembackup maken?')) return;
    return performBackupAction('Systeembackup starten...', async () => {
      const response = await fetch(`${apiBaseUrl}/api/v1/system/backup/runs`, { method: 'POST' });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || 'Systeembackup starten mislukt');
      return data;
    }, 'Systeembackup wordt gemaakt.');
  }
  if (action === 'snapshots') return loadBackupSnapshots();
  if (action === 'restore') return restoreBackupSnapshot();
}

async function loadBackupSnapshots() {
  const recovery = document.querySelector('#backup-recovery');
  if (!recovery) return;
  const status = recovery.querySelector('#backup-recovery-status');
  const select = recovery.querySelector('#backup-recovery-snapshot');
  const refresh = recovery.querySelector('[data-backup-action="snapshots"]');
  const restore = recovery.querySelector('[data-backup-action="restore"]');
  refresh.disabled = true;
  select.disabled = true;
  restore.disabled = true;
  status.className = 'system-config-status';
  status.textContent = 'Beschikbare snapshots laden...';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/system/backup/snapshots`);
    const data = await response.json().catch(() => null);
    if (!response.ok) throw new Error(backupValidationMessage(data?.error, 'Snapshots konden niet worden geladen.'));
    if (!Array.isArray(data)) throw new Error('De lijst met snapshots heeft een ongeldig formaat.');
    select.innerHTML = data.length ? data.map((snapshot) => {
      const id = String(snapshot.id || '');
      const shortId = String(snapshot.shortId || id);
      return `<option value="${escapeHtml(id)}">${formatBackupTimestamp(snapshot.time)} - ${escapeHtml(shortId)}</option>`;
    }).join('') : '<option value="">Geen snapshots beschikbaar</option>';
    select.disabled = data.length === 0;
    restore.disabled = data.length === 0;
    refresh.textContent = 'Snapshots vernieuwen';
    status.textContent = data.length ? `${data.length} snapshot${data.length === 1 ? '' : 's'} geladen. Kies zorgvuldig het juiste herstelpunt.` : 'Er zijn geen snapshots beschikbaar om te herstellen.';
  } catch (error) {
    select.innerHTML = '<option value="">Snapshots konden niet worden geladen</option>';
    status.className = 'system-config-status error';
    status.textContent = error.message;
  } finally {
    refresh.disabled = false;
  }
}

async function restoreBackupSnapshot() {
  const recovery = document.querySelector('#backup-recovery');
  if (!recovery) return;
  const status = recovery.querySelector('#backup-recovery-status');
  const select = recovery.querySelector('#backup-recovery-snapshot');
  const snapshotId = select.value;
  if (!snapshotId) {
    status.className = 'system-config-status error';
    status.textContent = 'Kies eerst een snapshot om te herstellen.';
    return;
  }
  if (!window.confirm('WAARSCHUWING: database, documenten en systeemconfiguratie worden vervangen. Weet u zeker dat u deze backup wilt herstellen?')) return;
  const confirmation = window.prompt('Deze actie kan niet ongedaan worden gemaakt. Typ exact HERSTEL om door te gaan.');
  if (confirmation !== 'HERSTEL') {
    status.className = 'system-config-status error';
    status.textContent = 'Herstel geannuleerd. U moet exact HERSTEL typen om door te gaan.';
    return;
  }
  recovery.querySelectorAll('button, select').forEach((control) => { control.disabled = true; });
  status.className = 'system-config-status';
  status.textContent = 'Herstel starten. Sluit dit venster niet...';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/system/backup/recovery`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ snapshotId, confirmation: 'HERSTEL' }),
    });
    const data = await response.json().catch(() => ({}));
    if (response.status !== 202) throw new Error(backupValidationMessage(data.error, 'Backup herstellen kon niet worden gestart.'));
    if (backupPollTimer) window.clearTimeout(backupPollTimer);
    backupPollTimer = null;
    document.querySelectorAll('#backup-configuration button, #backup-configuration input, #backup-configuration select, #backup-configuration textarea').forEach((control) => { control.disabled = true; });
    status.className = 'system-config-status backup-recovery-accepted';
    status.textContent = 'Herstel geaccepteerd. De applicatie wordt opnieuw gestart; deze pagina is tijdelijk niet bereikbaar.';
  } catch (error) {
    status.className = 'system-config-status error';
    status.textContent = error.message;
    recovery.querySelector('[data-backup-action="snapshots"]').disabled = false;
    select.disabled = false;
    recovery.querySelector('[data-backup-action="restore"]').disabled = !select.value;
  }
}

function backupValidationMessage(code, fallback = 'Doelopslag of repository kon niet worden gevalideerd.') {
  const messages = {
    proton_username_invalid: 'Proton Drive herkent dit account niet. Gebruik het primaire Proton-loginadres, niet een alias of eigen domein.',
    proton_2fa_required: 'Proton Drive vereist 2FA. Voeg een geldige 2fa-code of obscured otp_secret_key toe aan de rclone-configuratie.',
    rclone_password_format: 'Het wachtwoord in de rclone-configuratie is niet geldig obscured.',
    restic_password_invalid: 'Het restic-wachtwoord hoort niet bij de bestaande repository.',
    destination_rate_limited: 'De opslagprovider weigert tijdelijk nieuwe aanvragen. Probeer het later opnieuw.',
    destination_authentication_failed: 'Aanmelden bij de doelopslag is mislukt. Controleer account, wachtwoord en eventuele 2FA.',
    rclone_destination_failed: 'Rclone kon het opgegeven doel niet openen of aanmaken.',
    restic_repository_failed: 'De restic-repository kon niet worden geopend of geïnitialiseerd.',
    backup_invalid_configuration: 'De backupconfiguratie is ongeldig of overlapt met de live opslag.',
    backup_running: 'De configuratie kan niet worden getest terwijl een backup actief is.',
    snapshot_not_found: 'De gekozen snapshot bestaat niet meer. Vernieuw de lijst en kies opnieuw.',
    recovery_running: 'Er wordt al een backup hersteld.',
    invalid_confirmation: 'De vereiste herstelbevestiging ontbreekt of is ongeldig.',
  };
  return messages[code] || fallback;
}

async function performBackupAction(progress, action, success) {
  const root = document.querySelector('#backup-configuration');
  const status = document.querySelector('#backup-config-status');
  root.querySelectorAll('button').forEach((button) => { button.disabled = true; });
  if (status) {
    status.className = 'system-config-status';
    status.textContent = progress;
  }
  try {
    const data = await action();
    root.innerHTML = renderBackupConfiguration(data, success);
    scheduleBackupPoll(data);
  } catch (error) {
    if (status) {
      status.className = 'system-config-status error';
      status.textContent = error.message;
    }
    root.querySelectorAll('button').forEach((button) => { button.disabled = false; });
    const run = root.querySelector('[data-backup-action="run"]');
    if (run) run.disabled = true;
    const restore = root.querySelector('[data-backup-action="restore"]');
    if (restore) restore.disabled = !root.querySelector('#backup-recovery-snapshot')?.value;
  }
}

function scheduleBackupPoll(data) {
  if (backupPollTimer) window.clearTimeout(backupPollTimer);
  if (!data.running) return;
  backupPollTimer = window.setTimeout(async () => {
    try {
      const response = await fetch(`${apiBaseUrl}/api/v1/system/backup`);
      const next = await response.json();
      if (response.ok && document.querySelector('#backup-configuration')) {
        document.querySelector('#backup-configuration').innerHTML = renderBackupConfiguration(next);
        scheduleBackupPoll(next);
      } else {
        scheduleBackupPoll({ running: true });
      }
    } catch (_) {
      scheduleBackupPoll({ running: true });
    }
  }, 2000);
}

async function openRelatedDomainDetail(event) {
  const button = event.target.closest('[data-related-domain-type][data-related-domain-id]');
  if (!button) return;
  const id = Number(button.dataset.relatedDomainId);
  const destination = {
    asset: { section: 'wealth', wealthSection: 'assets', load: loadAssetDetail },
    liability: { section: 'wealth', wealthSection: 'liabilities', load: loadLiabilityDetail },
    case: { section: 'cases', load: loadCaseDetail },
  }[button.dataset.relatedDomainType];
  if (!destination || !Number.isSafeInteger(id) || id <= 0) return;
  activateMainSection(destination.section, destination.wealthSection);
  await destination.load(id);
}

async function addMainCategory(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const input = form.elements.name;

  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/main-categories`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: input.value }),
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || 'Hoofdcategorie toevoegen mislukt');
    }

    mainCategoriesState.push(data);
    casesLoaded = false;
    document.querySelector('#main-category-list').innerHTML = renderMainCategoryList();
    form.reset();
  } catch (error) {
    window.alert(error.message);
  }
}

async function handleMainCategoryAction(event) {
  const button = event.target.closest('button[data-action]');
  if (!button) {
    return;
  }

  const id = Number(button.dataset.id);
  if (button.dataset.action === 'edit-main') {
    renderSubcategoryEditor(id);
    return;
  }

  if (button.dataset.action === 'delete-main') {
    await deleteMainCategory(id);
  }
}

async function handleSubcategoryAction(event) {
  const button = event.target.closest('button[data-action]');
  if (!button) {
    return;
  }

  const id = Number(button.dataset.id);
  if (button.dataset.action === 'rename-main') {
    await renameMainCategory(id);
  }
  if (button.dataset.action === 'delete-subcategory') {
    await deleteSubcategory(id);
  }
  if (button.dataset.action === 'edit-subcategory') {
    editSubcategory(id);
  }
  if (button.dataset.action === 'cancel-subcategory-edit') {
    resetSubcategoryForm();
  }
}

async function renameMainCategory(id) {
  const current = mainCategoriesState.find((category) => category.id === id);
  const name = window.prompt('Nieuwe naam hoofdcategorie', current?.name || '');
  if (!name) {
    return;
  }

  const response = await fetch(`${apiBaseUrl}/api/v1/main-categories/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  const data = await response.json();
  if (!response.ok) {
    window.alert(data.error || 'Hoofdcategorie wijzigen mislukt');
    return;
  }

  mainCategoriesState = mainCategoriesState.map((category) => category.id === id ? data : category);
  casesLoaded = false;
  document.querySelector('#main-category-list').innerHTML = renderMainCategoryList();
  renderSubcategoryEditor(id);
}

async function deleteMainCategory(id) {
  const current = mainCategoriesState.find((category) => category.id === id);
  if (!window.confirm(`Hoofdcategorie "${current?.name || ''}" en de gekoppelde registratie verwijderen? Dit kan alleen als er geen subcategorieen of koppelingen zijn.`)) {
    return;
  }

  const response = await fetch(`${apiBaseUrl}/api/v1/main-categories/${id}`, { method: 'DELETE' });
  if (!response.ok) {
    const data = await response.json();
    window.alert(data.error || 'Hoofdcategorie verwijderen mislukt');
    return;
  }

  mainCategoriesState = mainCategoriesState.filter((category) => category.id !== id);
  casesLoaded = false;
  categoriesState = categoriesState.filter((category) => category.mainCategoryId !== id);
  document.querySelector('#main-category-list').innerHTML = renderMainCategoryList();
  refreshCategorySummaries();
  document.querySelector('#subcategory-editor').innerHTML = '<strong>Kies een hoofdcategorie</strong><p>Klik op wijzigen naast een hoofdcategorie om subcategorieen te onderhouden.</p>';
}

async function saveSubcategory(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const mainCategoryId = Number(form.dataset.mainCategoryId);
  const id = Number(form.elements.id.value);
  const response = await fetch(id ? `${apiBaseUrl}/api/v1/categories/${id}` : `${apiBaseUrl}/api/v1/categories`, {
    method: id ? 'PATCH' : 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      mainCategoryId,
      name: form.elements.name.value,
      kind: form.elements.kind.value,
      purpose: form.elements.purpose.value,
    }),
  });
  const data = await response.json();
  if (!response.ok) {
    window.alert(data.error || 'Subcategorie opslaan mislukt');
    return;
  }

  if (id) {
    categoriesState = categoriesState.map((category) => category.id === id ? data : category);
  } else {
    categoriesState.push(data);
  }
  refreshCategorySummaries();
  renderSubcategoryEditor(data.mainCategoryId);
}

function editSubcategory(id) {
  const category = categoriesState.find((item) => item.id === id);
  const form = document.querySelector('#subcategory-form');
  if (!category || !form) {
    return;
  }

  form.elements.id.value = category.id;
  form.elements.name.value = category.name;
  form.elements.kind.value = category.kind;
  form.elements.purpose.value = category.purpose;
  document.querySelector('#subcategory-submit').textContent = 'Subcategorie opslaan';
  document.querySelector('#subcategory-cancel').classList.remove('hidden');
  form.elements.name.focus();
}

function resetSubcategoryForm() {
  const form = document.querySelector('#subcategory-form');
  if (!form) {
    return;
  }
  const mainCategoryId = form.dataset.mainCategoryId;
  form.reset();
  form.dataset.mainCategoryId = mainCategoryId;
  form.elements.id.value = '';
  document.querySelector('#subcategory-submit').textContent = 'Subcategorie toevoegen';
  document.querySelector('#subcategory-cancel').classList.add('hidden');
}

function handleAccountAction(event) {
  const button = event.target.closest('button[data-action]');
  if (!button) {
    return;
  }

  const id = Number(button.dataset.id);
  if (button.dataset.action === 'edit-account') {
    editAccount(id);
  }
  if (button.dataset.action === 'delete-account') {
    deleteAccount(id);
  }
  if (button.dataset.action === 'cancel-account-edit') {
    resetAccountForm();
  }
}

async function saveAccount(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const id = Number(form.elements.id.value);
  const response = await fetch(id ? `${apiBaseUrl}/api/v1/accounts/${id}` : `${apiBaseUrl}/api/v1/accounts`, {
    method: id ? 'PATCH' : 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: form.elements.name.value,
      iban: form.elements.iban.value,
      type: form.elements.type.value,
      owner: form.elements.owner.value,
      bankProfile: form.elements.bankProfile.value,
    }),
  });
  const data = await response.json();
  if (!response.ok) {
    window.alert(data.error || 'Bankrekening opslaan mislukt');
    return;
  }

  if (id) {
    accountsState = accountsState.map((account) => account.id === id ? data : account);
  } else {
    accountsState.push(data);
  }
  refreshAccounts();
  resetAccountForm();
}

function editAccount(id) {
  const account = accountsState.find((item) => item.id === id);
  const form = document.querySelector('#account-form');
  if (!account || !form) {
    return;
  }

  form.elements.id.value = account.id;
  form.elements.name.value = account.name;
  form.elements.iban.value = account.iban || '';
  form.elements.type.value = account.type;
  form.elements.owner.value = account.owner || '';
  form.elements.bankProfile.value = account.bankProfile || 'generic';
  document.querySelector('#account-submit').textContent = 'Rekening opslaan';
  document.querySelector('#account-cancel').classList.remove('hidden');
  form.elements.name.focus();
}

async function deleteAccount(id) {
  const account = accountsState.find((item) => item.id === id);
  if (!window.confirm(`Bankrekening "${account?.name || ''}" verwijderen?`)) {
    return;
  }

  const response = await fetch(`${apiBaseUrl}/api/v1/accounts/${id}`, { method: 'DELETE' });
  if (!response.ok) {
    const data = await response.json();
    window.alert(data.error || 'Bankrekening verwijderen mislukt');
    return;
  }

  accountsState = accountsState.filter((item) => item.id !== id);
  refreshAccounts();
  resetAccountForm();
}

function resetAccountForm() {
  const form = document.querySelector('#account-form');
  if (!form) {
    return;
  }
  form.reset();
  form.elements.id.value = '';
  document.querySelector('#account-submit').textContent = 'Rekening toevoegen';
  document.querySelector('#account-cancel').classList.add('hidden');
}

function refreshAccounts() {
  document.querySelector('#account-list').innerHTML = renderAccountList();
  const dashboardSelect = document.querySelector('#dashboard-account-select');
  if (dashboardSelect) {
    dashboardSelect.innerHTML = renderTransactionAccountOptions();
    loadDashboardTransactions();
  }
  const manualSelect = document.querySelector('#manual-transaction-form select[name="accountId"]');
  if (manualSelect) {
    manualSelect.innerHTML = renderManualAccountOptions();
    manualSelect.disabled = accountsState.length === 0;
  }
  const accountSelect = document.querySelector('#import-form select[name="accountId"]');
  if (accountSelect) {
    accountSelect.innerHTML = renderImportAccountOptions();
    accountSelect.disabled = accountsState.length === 0;
  }
}

async function deleteSubcategory(id) {
  const current = categoriesState.find((category) => category.id === id);
  if (!window.confirm(`Subcategorie "${current?.name || ''}" verwijderen?`)) {
    return;
  }

  const response = await fetch(`${apiBaseUrl}/api/v1/categories/${id}`, { method: 'DELETE' });
  if (!response.ok) {
    const data = await response.json();
    window.alert(data.error || 'Subcategorie verwijderen mislukt');
    return;
  }

  categoriesState = categoriesState.filter((category) => category.id !== id);
  refreshCategorySummaries();
  renderSubcategoryEditor(current.mainCategoryId);
}

function refreshCategorySummaries() {
  document.querySelectorAll('[data-category-summary]').forEach((list) => {
    list.innerHTML = categoriesFor(categoriesState, list.dataset.categorySummary);
  });
}

async function loadBankProfiles() {
  const response = await fetch(`${apiBaseUrl}/api/v1/imports/bank-profiles`);
  if (!response.ok) {
    return ['generic'];
  }
  const data = await response.json();
  return data.profiles || ['generic'];
}

async function previewImport(event) {
  event.preventDefault();
  const result = document.querySelector('#import-result');
  const formData = new FormData(event.currentTarget);
  const action = event.submitter?.dataset.importAction || 'preview';
  const account = accountsState.find((item) => item.id === Number(formData.get('accountId')));
  if (!account) {
    result.innerHTML = '<p class="error">Kies eerst een bankrekening voor deze import.</p>';
    return;
  }

  formData.set('profile', account.bankProfile || 'generic');
  result.innerHTML = action === 'commit' ? '<p>Importbestand wordt opgeslagen...</p>' : '<p>Importbestand wordt gelezen...</p>';

  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/imports${action === 'commit' ? '' : '/preview'}`, {
      method: 'POST',
      body: formData,
    });

    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || 'Import preview mislukt');
    }

    if (action === 'commit') {
      result.innerHTML = renderCommitResult(data);
      incrementTransactionSummary(data.result.imported);
      await refreshTransactionYears();
      return;
    }

    result.innerHTML = renderImportResult(data);
  } catch (error) {
    result.innerHTML = `<p class="error">${error.message}</p>`;
  }
}

function renderCommitResult(data) {
  return `
    <p><strong>${data.result.imported}</strong> transacties opgeslagen voor <strong>${escapeHtml(data.account.name)}</strong>.</p>
    <p>${data.result.duplicates} dubbele transacties overgeslagen van ${data.result.total} gelezen regels.</p>
    ${data.warnings.length ? `<p class="warning">${data.warnings.length} regels overgeslagen.</p>` : ''}
  `;
}

function incrementTransactionSummary(imported) {
  const element = document.querySelector('[data-summary="transactions"]');
	if (!element) {
		return;
	}
  element.textContent = String(Number(element.textContent) + imported);
}

function renderImportResult(preview) {
  const rows = preview.transactions
    .slice(0, 25)
    .map((transaction) => `
      <tr>
        <td>${escapeHtml(transaction.bookedOn)}</td>
        <td>${escapeHtml(transaction.description)}</td>
        <td>${escapeHtml(transaction.counterparty || '')}</td>
        <td class="amount">${formatAmount(transaction.amountCents)}</td>
      </tr>
    `)
    .join('');

  return `
    <p><strong>${preview.transactions.length}</strong> transacties herkend met profiel <strong>${preview.profile}</strong>.</p>
    ${preview.warnings.length ? `<p class="warning">${preview.warnings.length} regels overgeslagen.</p>` : ''}
    <div class="table-wrap">
      <table>
        <thead><tr><th>Datum</th><th>Omschrijving</th><th>Tegenpartij</th><th>Bedrag</th></tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
  `;
}

function formatAmount(cents) {
  return new Intl.NumberFormat('nl-NL', { style: 'currency', currency: 'EUR' }).format(cents / 100);
}

function escapeHtml(value) {
  return String(value).replace(/[&<>'"]/g, (character) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    "'": '&#39;',
    '"': '&quot;',
  }[character]));
}

Promise.all([loadSummary(), loadMainCategories(), loadCategories(), loadAccounts(), loadBankProfiles(), loadTransactionYears()])
  .then(([summary, mainCategories, categories, accounts, profiles, transactionYears]) => {
    bankProfiles = profiles;
    return { summary, mainCategories, categories, accounts, transactionYears };
  })
  .then(render)
  .catch((error) => {
    app.innerHTML = `<section class="shell"><h1>Niet verbonden</h1><p>${error.message}</p></section>`;
  });
