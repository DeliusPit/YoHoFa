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
let selectedAssetState = null;
let assetValuationsState = [];
let assetRelationDocumentsState = [];
let assetRelationTransactionsState = [];
let assetRelationLiabilitiesState = [];
let assetsLoaded = false;
let assetDetailRequestID = 0;
let liabilitiesState = [];
let selectedLiabilityState = null;
let liabilityRelationAssetsState = [];
let liabilityRelationDocumentsState = [];
let liabilityRelationTransactionsState = [];
let liabilitiesLoaded = false;
let liabilityDetailRequestID = 0;
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
        <button type="button" class="main-tab" data-main-target="transactions">Transacties</button>
        <button type="button" class="main-tab" data-main-target="assets">Bezittingen</button>
        <button type="button" class="main-tab" data-main-target="liabilities">Schulden</button>
        <button type="button" class="main-tab" data-main-target="cases">Dossiers</button>
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
          <details class="dashboard-panel" open>
            <summary><span>Rekeningen</span><strong data-summary="accounts">${accounts.length}</strong></summary>
            <ul class="dashboard-account-list">${renderDashboardAccountList()}</ul>
          </details>
        </div>
        <section class="dashboard-category-transactions transaction-panel" aria-label="Dashboardtransacties">
          <div class="panel-heading">
            <h2>Transacties</h2>
            <span id="dashboard-transaction-label">Kies een grafiekrubriek of rekening.</span>
          </div>
          <div id="dashboard-transaction-result" class="table-wrap"><p>Kies een grafiekrubriek of rekening.</p></div>
        </section>
      </section>

      <section class="app-tab-section" data-main-section="transactions" aria-label="Transacties">
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

      <section class="app-tab-section" data-main-section="assets" aria-label="Bezittingen">
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
                  <label>Valuta <input name="currency" type="text" maxlength="3" value="EUR" required /></label>
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
                  <button type="button" class="asset-detail-tab" data-asset-detail-target="documents">Documenten</button>
                   <button type="button" class="asset-detail-tab" data-asset-detail-target="transactions">Transacties</button>
                   <button type="button" class="asset-detail-tab" data-asset-detail-target="relations">Relaties</button>
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
                <section class="asset-detail-section" data-asset-detail-section="relations">
                  <form id="asset-relation-form" class="asset-relation-form">
                    <label>Objecttype <select name="targetType"><option value="transaction">Transactie</option><option value="document">Document</option><option value="liability">Schuld</option></select></label>
                    <label>Relatietype <select name="relationType"><option value="relates_to">Heeft betrekking op</option></select></label>
                    <label class="asset-relation-target">Gerelateerd object <select name="targetId" required></select></label>
                    <label>Geldig vanaf <input name="validFrom" type="date" /></label><label>Geldig tot <input name="validTo" type="date" /></label>
                    <label class="asset-relation-notes">Notitie <input name="notes" type="text" /></label>
                    <button type="submit">Relatie toevoegen</button>
                  </form>
                   <div id="asset-relations" class="asset-related-list"></div>
                 </section>
                 <section class="asset-detail-section" data-asset-detail-section="timeline"><div id="asset-timeline" class="domain-timeline"></div></section>
              </section>
              <section id="asset-empty-state" class="asset-panel empty-state"><strong>Kies een bezitting</strong><p>Selecteer een bezitting of maak een nieuwe aan.</p></section>
            </div>
          </div>
        </section>
      </section>

      <section class="app-tab-section" data-main-section="liabilities" aria-label="Schulden">
        <section id="liabilities-root" class="assets-shell liabilities-shell">
          <header class="assets-heading">
            <div><p class="eyebrow">Verplichtingen</p><h1>Schulden</h1><p>Beheer schulden en hun documenten, transacties en relaties.</p></div>
            <button type="button" class="asset-new-button" data-liability-action="new">Nieuwe schuld</button>
          </header>
          <form id="liability-filter-form" class="asset-filter-form">
            <label>Type <select name="type"><option value="">Alle typen</option>${liabilityTypeOptions()}</select></label>
            <label>Status <select name="status"><option value="">Alle statussen</option>${liabilityStatusOptions()}</select></label>
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
                  <label>Valuta <input name="currency" type="text" maxlength="3" value="EUR" required /></label>
                  <label class="asset-description-field">Omschrijving <textarea name="description" rows="3"></textarea></label>
                  <div class="form-actions asset-form-actions"><button type="submit">Opslaan</button><button type="button" class="secondary-button" data-liability-action="cancel-form">Annuleren</button></div>
                </form>
              </section>
              <section id="liability-detail" class="asset-panel hidden">
                <div class="asset-detail-heading"><div><small>Schuld</small><h2 id="liability-detail-name"></h2></div><div class="row-actions"><button type="button" class="secondary-button" data-liability-action="edit">Wijzigen</button><button type="button" class="secondary-button" data-liability-action="delete">Verwijderen</button></div></div>
                <nav class="asset-detail-tabs" aria-label="Schuld onderdelen">
                  <button type="button" class="asset-detail-tab active" data-liability-detail-target="overview">Overzicht</button>
                  <button type="button" class="asset-detail-tab" data-liability-detail-target="documents">Documenten</button>
                   <button type="button" class="asset-detail-tab" data-liability-detail-target="transactions">Transacties</button>
                   <button type="button" class="asset-detail-tab" data-liability-detail-target="relations">Relaties</button>
                   <button type="button" class="asset-detail-tab" data-liability-detail-target="timeline">Tijdlijn</button>
                </nav>
                <section class="asset-detail-section active" data-liability-detail-section="overview" id="liability-overview"></section>
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
                <section class="asset-detail-section" data-liability-detail-section="relations">
                  <form id="liability-relation-form" class="asset-relation-form">
                    <label>Objecttype <select name="targetType"><option value="asset">Bezitting</option><option value="transaction">Transactie</option><option value="document">Document</option></select></label>
                    <label>Relatietype <select name="relationType"><option value="financed_by">Gefinancierd door</option></select></label>
                    <label class="asset-relation-target">Gerelateerd object <select name="targetId" required></select></label>
                    <label>Geldig vanaf <input name="validFrom" type="date" /></label><label>Geldig tot <input name="validTo" type="date" /></label>
                    <label class="asset-relation-notes">Notitie <input name="notes" type="text" /></label>
                    <button type="submit">Relatie toevoegen</button>
                  </form>
                   <div id="liability-relations" class="asset-related-list"></div>
                 </section>
                 <section class="asset-detail-section" data-liability-detail-section="timeline"><div id="liability-timeline" class="domain-timeline"></div></section>
              </section>
              <section id="liability-empty-state" class="asset-panel empty-state"><strong>Kies een schuld</strong><p>Selecteer een schuld of maak een nieuwe aan.</p></section>
            </div>
          </div>
        </section>
      </section>

      <section class="app-tab-section" data-main-section="cases" aria-label="Dossiers">
        <section id="cases-root" class="assets-shell cases-shell">
          <header class="assets-heading dossier-heading">
            <div><p class="eyebrow">Levensgebeurtenissen</p><h1>Dossiers</h1><p>Bundel gebeurtenissen, documenten, transacties en gerelateerde objecten.</p></div>
            <button type="button" class="asset-new-button" data-case-action="new">Nieuw dossier</button>
          </header>
          <form id="case-filter-form" class="asset-filter-form case-filter-form">
            <label>Type <input name="caseType" type="search" placeholder="Bijvoorbeeld tax_return" /></label>
            <label>Status <select name="status"><option value="">Alle statussen</option>${caseStatusOptions()}</select></label>
          </form>
          <div class="assets-layout">
            <aside class="asset-list-panel"><div id="case-list"><p>Open Dossiers om de lijst te laden.</p></div></aside>
            <div class="asset-content">
              <section id="case-form-panel" class="asset-panel hidden">
                <div class="panel-heading"><h2 id="case-form-title">Nieuw dossier</h2></div>
                <form id="case-form" class="asset-form case-form">
                  <input name="id" type="hidden" />
                  <label>Titel <input name="title" type="text" required /></label>
                  <label>Type <input name="caseType" type="text" list="case-type-examples" required /></label>
                  <datalist id="case-type-examples"><option value="tax_return"></option><option value="home_purchase"></option><option value="renovation"></option><option value="estate"></option><option value="insurance_claim"></option><option value="emigration"></option><option value="other"></option></datalist>
                  <label>Startdatum <input name="startDate" type="date" /></label>
                  <label>Einddatum <input name="endDate" type="date" /></label>
                  <label>Status <select name="status" required>${caseStatusOptions('active')}</select></label>
                  <label class="asset-description-field">Omschrijving <textarea name="description" rows="4"></textarea></label>
                  <div class="form-actions asset-form-actions"><button type="submit">Opslaan</button><button type="button" class="secondary-button" data-case-action="cancel-form">Annuleren</button></div>
                </form>
              </section>
              <section id="case-detail" class="asset-panel hidden">
                <div class="asset-detail-heading"><div><small>Dossier</small><h2 id="case-detail-title"></h2></div><div class="row-actions"><button type="button" class="secondary-button" data-case-action="edit">Wijzigen</button><button type="button" class="secondary-button" data-case-action="delete">Verwijderen</button></div></div>
                <nav class="asset-detail-tabs" aria-label="Dossier onderdelen">
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
              <section id="case-empty-state" class="asset-panel empty-state"><strong>Kies een dossier</strong><p>Selecteer een dossier of maak een nieuw dossier aan.</p></section>
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
          <nav class="config-menu" aria-label="Configuratie onderdelen">
            <button type="button" class="config-tab active" data-config-target="categories">Onderhoud categorieen</button>
            <button type="button" class="config-tab" data-config-target="accounts">Onderhoud bankrekeningen</button>
            <button type="button" class="config-tab" data-config-target="hidden-transactions">Verborgen transacties</button>
          </nav>
          <section class="config-section active" data-config-section="categories" aria-label="Hoofdcategorieen beheren">
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
          <section class="config-section" data-config-section="accounts" aria-label="Bankrekeningen beheren">
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
          <section class="config-section" data-config-section="hidden-transactions" aria-label="Verborgen transacties beheren">
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
  document.querySelector('.dashboard-account-list').addEventListener('click', handleDashboardAccountSelection);
  document.querySelector('#transaction-result').addEventListener('click', handleAccountPagination);
  document.querySelector('#category-transaction-filter-form select[name="mainCategoryId"]').addEventListener('change', updateCategoryTransactionSubcategories);
  document.querySelector('#category-transaction-filter-form select[name="categoryId"]').addEventListener('change', resetAndLoadDashboardTransactions);
  document.querySelectorAll('.main-tab').forEach((button) => button.addEventListener('click', switchMainSection));
  document.querySelectorAll('.config-tab').forEach((button) => button.addEventListener('click', switchConfigSection));
  document.querySelector('#import-form').addEventListener('submit', previewImport);
  document.querySelector('#document-upload-form').addEventListener('submit', uploadTransactionDocument);
  document.querySelector('#document-link-form').addEventListener('submit', linkExistingDocument);
  document.querySelector('#transaction-document-list').addEventListener('click', unlinkTransactionDocument);
  document.querySelector('#assets-root').addEventListener('click', handleAssetAction);
  document.querySelector('#asset-filter-form').addEventListener('change', loadAssets);
  document.querySelector('#asset-form').addEventListener('submit', saveAsset);
  document.querySelector('#asset-valuation-form').addEventListener('submit', saveAssetValuation);
  document.querySelector('#asset-document-upload-form').addEventListener('submit', uploadAssetDocument);
  document.querySelector('#asset-document-link-form').addEventListener('submit', linkExistingAssetDocument);
  document.querySelector('#asset-transaction-link-form').addEventListener('submit', linkAssetTransaction);
  document.querySelector('#asset-relation-form').addEventListener('submit', saveAssetRelation);
  document.querySelector('#asset-relation-form select[name="targetType"]').addEventListener('change', updateAssetRelationForm);
  document.querySelector('#liabilities-root').addEventListener('click', handleLiabilityAction);
  document.querySelector('#liability-filter-form').addEventListener('change', loadLiabilities);
  document.querySelector('#liability-form').addEventListener('submit', saveLiability);
  document.querySelector('#liability-document-upload-form').addEventListener('submit', uploadLiabilityDocument);
  document.querySelector('#liability-document-link-form').addEventListener('submit', linkExistingLiabilityDocument);
  document.querySelector('#liability-transaction-link-form').addEventListener('submit', linkLiabilityTransaction);
  document.querySelector('#liability-relation-form').addEventListener('submit', saveLiabilityRelation);
  document.querySelector('#liability-relation-form select[name="targetType"]').addEventListener('change', updateLiabilityRelationForm);
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

function renderDashboardAccountList() {
  if (!accountsState.length) {
    return '<li><span>Nog geen bankrekeningen</span></li>';
  }
  return accountsState.map((account) => `
    <li>
      <button type="button" class="dashboard-account-button ${dashboardTransactionSelection?.type === 'account' && dashboardTransactionSelection.id === account.id ? 'active' : ''}" data-dashboard-account="${account.id}" data-dashboard-label="${escapeHtml(account.name)}">
        <span>${escapeHtml(account.name)}</span>
        <small>${escapeHtml(account.iban || (account.type === 'savings' ? 'Spaarrekening' : 'Betaalrekening'))} / ${formatAmount(account.balanceCents || 0)}</small>
      </button>
    </li>
  `).join('');
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

function handleDashboardAccountSelection(event) {
  const button = event.target.closest('button[data-dashboard-account]');
  if (!button) {
    return;
  }
  selectDashboardTransactions({ type: 'account', id: Number(button.dataset.dashboardAccount), label: button.dataset.dashboardLabel });
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
  document.querySelectorAll('button[data-dashboard-account]').forEach((button) => {
    button.classList.toggle('active', dashboardTransactionSelection?.type === 'account' && dashboardTransactionSelection.id === Number(button.dataset.dashboardAccount));
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
      <div><a href="${apiBaseUrl}/api/v1/documents/${document.id}/content" target="_blank" rel="noopener">${escapeHtml(document.originalFilename)}</a><small>${escapeHtml(document.documentDate || document.mimeType)} &middot; ${formatFileSize(document.sizeBytes)}</small>${document.description ? `<span>${escapeHtml(document.description)}</span>` : ''}</div>
      <button type="button" class="icon-button danger" data-action="unlink-document" data-document-id="${document.id}" aria-label="Verwijder koppeling">&#10005;</button>
    </li>
  `).join('')}</ul>`;
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
    const response = await fetch(`${apiBaseUrl}/api/v1/assets?${params}`);
    const assets = await response.json();
    if (!response.ok) throw new Error(assets.error || 'Bezittingen laden mislukt');
    assetsState = assets;
    assetsLoaded = true;
    list.innerHTML = renderAssetList(assets);
  } catch (error) {
    list.innerHTML = `<p class="error">${escapeHtml(error.message)}</p>`;
  }
}

function renderAssetList(assets) {
  if (!assets.length) return '<p>Geen bezittingen gevonden.</p>';
  return `<ul>${assets.map((asset) => `
    <li><button type="button" class="asset-list-button ${selectedAssetState?.id === asset.id ? 'active' : ''}" data-asset-id="${asset.id}"><span><strong>${escapeHtml(asset.name)}</strong><small>${escapeHtml(assetTypeLabel(asset.assetType))} · ${escapeHtml(assetStatusLabel(asset.status))}</small></span><b>${formatAssetValue(asset.currentValueCents, asset.currency)}</b></button></li>
  `).join('')}</ul>`;
}

async function handleAssetAction(event) {
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
  case 'delete-entity-relation':
    await deleteAssetRelation(Number(button.dataset.relationId));
    break;
  }
}

function showAssetForm(asset = null) {
  assetDetailRequestID += 1;
  const form = document.querySelector('#asset-form');
  form.reset();
  form.elements.id.value = asset?.id || '';
  form.elements.name.value = asset?.name || '';
  form.elements.assetType.value = asset?.assetType || 'real_estate';
  form.elements.status.value = asset?.status || 'active';
  form.elements.currency.value = asset?.currency || 'EUR';
  form.elements.acquiredAt.value = asset?.acquiredAt || '';
  form.elements.disposedAt.value = asset?.disposedAt || '';
  form.elements.purchaseValue.value = centsToInput(asset?.purchaseValueCents || 0);
  form.elements.description.value = asset?.description || '';
  document.querySelector('#asset-form-title').textContent = asset ? 'Bezitting wijzigen' : 'Nieuwe bezitting';
  document.querySelector('#asset-form-panel').classList.remove('hidden');
  document.querySelector('#asset-detail').classList.add('hidden');
  document.querySelector('#asset-empty-state').classList.add('hidden');
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
    currency: form.elements.currency.value,
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
  try {
    const [assetResponse, valuationsResponse, documentsResponse, transactionsResponse, relationsResponse, allDocumentsResponse, liabilitiesResponse, eventsResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/assets/${id}`), fetch(`${apiBaseUrl}/api/v1/assets/${id}/valuations`),
      fetch(`${apiBaseUrl}/api/v1/assets/${id}/documents`), fetch(`${apiBaseUrl}/api/v1/assets/${id}/transactions`),
      fetch(`${apiBaseUrl}/api/v1/assets/${id}/relations`), fetch(`${apiBaseUrl}/api/v1/documents`),
      fetch(`${apiBaseUrl}/api/v1/liabilities`), fetch(`${apiBaseUrl}/api/v1/assets/${id}/events`),
    ]);
    const [asset, valuations, documentsResult, transactions, relations, allDocumentsResult, liabilities, eventsResult] = await Promise.all([assetResponse.json(), valuationsResponse.json(), documentsResponse.json(), transactionsResponse.json(), relationsResponse.json(), allDocumentsResponse.json(), liabilitiesResponse.json(), eventsResponse.json()]);
    const documentsUnavailable = documentsResponse.status === 501 || allDocumentsResponse.status === 501;
    if (!assetResponse.ok || !valuationsResponse.ok || (!documentsResponse.ok && documentsResponse.status !== 501) || !transactionsResponse.ok || !relationsResponse.ok || (!allDocumentsResponse.ok && allDocumentsResponse.status !== 501) || !liabilitiesResponse.ok || !eventsResponse.ok) throw new Error(asset.error || valuations.error || documentsResult.error || transactions.error || relations.error || allDocumentsResult.error || liabilities.error || eventsResult?.error || 'Bezitting laden mislukt');
    const events = validateDomainEvents(eventsResult);
    if (requestID !== assetDetailRequestID) return;
    const documents = documentsResponse.status === 501 ? [] : documentsResult;
    const allDocuments = allDocumentsResponse.status === 501 ? [] : allDocumentsResult;
    selectedAssetState = asset;
    assetValuationsState = valuations;
    assetRelationDocumentsState = allDocuments;
    assetRelationLiabilitiesState = liabilities;
    document.querySelector('#asset-detail-name').textContent = asset.name;
    document.querySelector('#asset-overview').innerHTML = renderAssetOverview(asset);
    document.querySelector('#asset-current-value').textContent = formatAssetValue(asset.currentValueCents, asset.currency);
    document.querySelector('#asset-valuation-list').innerHTML = renderAssetValuations(valuations, asset.currency);
    resetAssetValuationForm();
    document.querySelector('#asset-document-list').innerHTML = renderAssetDocuments(documents);
    document.querySelector('#asset-document-status').textContent = documentsUnavailable ? 'Documentopslag is momenteel niet beschikbaar.' : '';
    renderAssetDocumentOptions(allDocuments, documents);
    document.querySelector('#asset-transaction-list').innerHTML = renderAssetTransactions(transactions);
    document.querySelector('#asset-relations').innerHTML = renderAssetRelations(relations);
    document.querySelector('#asset-timeline').innerHTML = renderDomainTimeline(events);
    document.querySelector('#asset-detail').classList.remove('hidden');
    document.querySelector('#asset-empty-state').classList.add('hidden');
    document.querySelector('#asset-form-panel').classList.add('hidden');
    document.querySelector('#asset-list').innerHTML = renderAssetList(assetsState);
    switchAssetDetailSection(targetSection);
    updateAssetRelationForm();
    await loadAssetTransactionCandidates();
  } catch (error) {
    if (requestID === assetDetailRequestID) window.alert(error.message);
  }
}

function renderAssetOverview(asset) {
  return `<dl class="asset-overview-grid"><div><dt>Type</dt><dd>${escapeHtml(assetTypeLabel(asset.assetType))}</dd></div><div><dt>Status</dt><dd>${escapeHtml(assetStatusLabel(asset.status))}</dd></div><div><dt>Aankoopdatum</dt><dd>${escapeHtml(asset.acquiredAt || 'Niet vastgelegd')}</dd></div><div><dt>Afvoerdatum</dt><dd>${escapeHtml(asset.disposedAt || 'Niet van toepassing')}</dd></div><div><dt>Aankoopwaarde</dt><dd>${formatAssetValue(asset.purchaseValueCents, asset.currency)}</dd></div><div><dt>Huidige waarde</dt><dd>${formatAssetValue(asset.currentValueCents, asset.currency)}</dd></div></dl><p class="asset-description">${escapeHtml(asset.description || 'Geen omschrijving.')}</p>`;
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
  return `<ul>${documents.map((document) => `<li><div><a href="${apiBaseUrl}/api/v1/documents/${document.id}/content" target="_blank" rel="noopener">${escapeHtml(document.originalFilename)}</a><small>${escapeHtml(document.documentDate || document.mimeType)} &middot; ${formatFileSize(document.sizeBytes)}</small></div><button type="button" class="icon-button danger" data-asset-action="unlink-asset-document" data-document-id="${document.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
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
    assetRelationTransactionsState = transactions;
    select.innerHTML = transactions.length ? `<option value="">Kies een transactie</option>${transactions.map((transaction) => `<option value="${transaction.id}">${escapeHtml(transaction.bookedOn)} · ${escapeHtml(transaction.description)} · ${formatAmount(transaction.amountCents)}</option>`).join('')}` : '<option value="">Geen transacties gevonden</option>';
    select.disabled = !transactions.length;
    if (document.querySelector('#asset-relation-form').elements.targetType.value === 'transaction') updateAssetRelationForm();
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

function renderAssetRelations(relations) {
  if (!relations.length) return '<p>Nog geen relaties vastgelegd.</p>';
  return `<ul>${relations.map((relation) => {
    const assetIsSource = relation.sourceType === 'asset' && relation.sourceId === selectedAssetState?.id;
    const relatedType = assetIsSource ? relation.targetType : relation.sourceType;
    const relatedLabel = assetIsSource ? relation.targetLabel : relation.sourceLabel;
    const period = relation.validFrom || relation.validTo ? `<small>${escapeHtml(relation.validFrom || '...')} t/m ${escapeHtml(relation.validTo || '...')}</small>` : '';
    return `<li><div><strong>${escapeHtml(relatedLabel)}</strong><small>${escapeHtml(entityTypeLabel(relatedType))} &middot; ${escapeHtml(relationTypeLabel(relation.relationType))}</small>${period}${relation.notes ? `<p>${escapeHtml(relation.notes)}</p>` : ''}</div><button type="button" class="icon-button danger" data-asset-action="delete-entity-relation" data-relation-id="${relation.id}" aria-label="Verwijder relatie">&#10005;</button></li>`;
  }).join('')}</ul>`;
}

function updateAssetRelationForm() {
  const form = document.querySelector('#asset-relation-form');
  const targetType = form.elements.targetType.value;
  const items = targetType === 'document' ? assetRelationDocumentsState : targetType === 'liability' ? assetRelationLiabilitiesState : assetRelationTransactionsState;
  form.elements.relationType.innerHTML = targetType === 'document'
    ? '<option value="evidence_for">Bewijs voor</option>'
    : targetType === 'liability' ? '<option value="financed_by">Gefinancierd door</option>' : '<option value="relates_to">Heeft betrekking op</option>';
  form.elements.targetId.innerHTML = items.length
    ? `<option value="">Kies een object</option>${items.map((item) => {
      if (targetType === 'document') return `<option value="${item.id}">${escapeHtml(item.originalFilename)}</option>`;
      if (targetType === 'liability') return `<option value="${item.id}">${escapeHtml(item.name)}</option>`;
      return `<option value="${item.id}">${escapeHtml(item.bookedOn)} &middot; ${escapeHtml(item.description)}</option>`;
    }).join('')}`
    : '<option value="">Geen objecten beschikbaar</option>';
  form.elements.targetId.disabled = !items.length;
}

async function saveAssetRelation(event) {
  event.preventDefault();
  if (!selectedAssetState) return;
  const form = event.currentTarget;
  const targetID = Number(form.elements.targetId.value);
  if (!targetID) return;
  const assetID = selectedAssetState.id;
  const input = {
    targetType: form.elements.targetType.value,
    targetId: targetID,
    relationType: form.elements.relationType.value,
    validFrom: form.elements.validFrom.value,
    validTo: form.elements.validTo.value,
    notes: form.elements.notes.value,
  };
  const response = await fetch(`${apiBaseUrl}/api/v1/assets/${assetID}/relations`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
  if (!response.ok) return window.alert((await response.json()).error || 'Relatie toevoegen mislukt');
  form.elements.validFrom.value = '';
  form.elements.validTo.value = '';
  form.elements.notes.value = '';
  await loadAssetDetail(assetID, 'relations');
}

async function deleteAssetRelation(relationID) {
  if (!selectedAssetState || !window.confirm('Deze relatie verwijderen?')) return;
  const assetID = selectedAssetState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/assets/${assetID}/relations/${relationID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Relatie verwijderen mislukt');
  await loadAssetDetail(assetID, 'relations');
}

function entityTypeLabel(value) {
  const labels = { asset: 'Bezitting', liability: 'Schuld', transaction: 'Transactie', document: 'Document', case: 'Dossier' };
  return Object.hasOwn(labels, value) ? labels[value] : value;
}

function relationTypeLabel(value) {
  return { relates_to: 'Heeft betrekking op', evidence_for: 'Bewijs voor', financed_by: 'Gefinancierd door' }[value] || value;
}

function formatAssetValue(cents, currency) {
  return new Intl.NumberFormat('nl-NL', { style: 'currency', currency: currency || 'EUR' }).format(cents / 100);
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
    list.innerHTML = renderLiabilityList(liabilities);
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
  case 'delete-relation': await deleteLiabilityRelation(Number(button.dataset.relationId)); break;
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
  form.elements.currency.value = liability?.currency || 'EUR';
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
    currency: form.elements.currency.value,
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
    const [liabilityResponse, documentsResponse, transactionsResponse, relationsResponse, allDocumentsResponse, assetsResponse, eventsResponse] = await Promise.all([
      fetch(`${apiBaseUrl}/api/v1/liabilities/${id}`), fetch(`${apiBaseUrl}/api/v1/liabilities/${id}/documents`),
      fetch(`${apiBaseUrl}/api/v1/liabilities/${id}/transactions`), fetch(`${apiBaseUrl}/api/v1/liabilities/${id}/relations`),
      fetch(`${apiBaseUrl}/api/v1/documents`), fetch(`${apiBaseUrl}/api/v1/assets`), fetch(`${apiBaseUrl}/api/v1/liabilities/${id}/events`),
    ]);
    const [liability, documentsResult, transactions, relations, allDocumentsResult, assets, eventsResult] = await Promise.all([liabilityResponse.json(), documentsResponse.json(), transactionsResponse.json(), relationsResponse.json(), allDocumentsResponse.json(), assetsResponse.json(), eventsResponse.json()]);
    const documentsUnavailable = documentsResponse.status === 501 || allDocumentsResponse.status === 501;
    if (!liabilityResponse.ok || (!documentsResponse.ok && documentsResponse.status !== 501) || !transactionsResponse.ok || !relationsResponse.ok || (!allDocumentsResponse.ok && allDocumentsResponse.status !== 501) || !assetsResponse.ok || !eventsResponse.ok) throw new Error(liability.error || documentsResult.error || transactions.error || relations.error || allDocumentsResult.error || assets.error || eventsResult?.error || 'Schuld laden mislukt');
    const events = validateDomainEvents(eventsResult);
    if (requestID !== liabilityDetailRequestID) return;
    const documents = documentsResponse.status === 501 ? [] : documentsResult;
    const allDocuments = allDocumentsResponse.status === 501 ? [] : allDocumentsResult;
    selectedLiabilityState = liability;
    liabilityRelationAssetsState = assets;
    liabilityRelationDocumentsState = allDocuments;
    liabilityRelationTransactionsState = transactions;
    document.querySelector('#liability-detail-name').textContent = liability.name;
    document.querySelector('#liability-overview').innerHTML = renderLiabilityOverview(liability);
    document.querySelector('#liability-document-list').innerHTML = renderLiabilityDocuments(documents);
    document.querySelector('#liability-document-status').textContent = documentsUnavailable ? 'Documentopslag is momenteel niet beschikbaar.' : '';
    renderLiabilityDocumentOptions(allDocuments, documents);
    document.querySelector('#liability-transaction-list').innerHTML = renderLiabilityTransactions(transactions);
    document.querySelector('#liability-relations').innerHTML = renderLiabilityRelations(relations, liability.id);
    document.querySelector('#liability-timeline').innerHTML = renderDomainTimeline(events);
    document.querySelector('#liability-detail').classList.remove('hidden');
    document.querySelector('#liability-empty-state').classList.add('hidden');
    document.querySelector('#liability-form-panel').classList.add('hidden');
    document.querySelector('#liability-list').innerHTML = renderLiabilityList(liabilitiesState);
    switchLiabilityDetailSection(targetSection);
    updateLiabilityRelationForm();
    await loadLiabilityTransactionCandidates();
  } catch (error) {
    if (requestID === liabilityDetailRequestID) window.alert(error.message);
  }
}

function renderLiabilityOverview(liability) {
  return `<dl class="asset-overview-grid"><div><dt>Type</dt><dd>${escapeHtml(liabilityTypeLabel(liability.liabilityType))}</dd></div><div><dt>Status</dt><dd>${escapeHtml(liabilityStatusLabel(liability.status))}</dd></div><div><dt>Schuldeiser</dt><dd>${escapeHtml(liability.creditor || 'Niet vastgelegd')}</dd></div><div><dt>Looptijd</dt><dd>${escapeHtml(liability.startDate || '...')} t/m ${escapeHtml(liability.endDate || '...')}</dd></div><div><dt>Oorspronkelijk bedrag</dt><dd>${formatAssetValue(liability.originalAmountCents, liability.currency)}</dd></div><div><dt>Huidig saldo</dt><dd>${formatAssetValue(liability.currentBalanceCents, liability.currency)}</dd></div></dl><p class="asset-description">${escapeHtml(liability.description || 'Geen omschrijving.')}</p>`;
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

function renderLiabilityDocuments(documents) {
  if (!documents.length) return '<p>Nog geen documenten gekoppeld.</p>';
  return `<ul>${documents.map((item) => `<li><div><a href="${apiBaseUrl}/api/v1/documents/${item.id}/content" target="_blank" rel="noopener">${escapeHtml(item.originalFilename)}</a><small>${escapeHtml(item.documentDate || item.mimeType)} &middot; ${formatFileSize(item.sizeBytes)}</small></div><button type="button" class="icon-button danger" data-liability-action="unlink-document" data-document-id="${item.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
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
    liabilityRelationTransactionsState = transactions;
    form.elements.transactionId.innerHTML = transactions.length ? `<option value="">Kies een transactie</option>${transactions.map((item) => `<option value="${item.id}">${escapeHtml(item.bookedOn)} · ${escapeHtml(item.description)} · ${formatAmount(item.amountCents)}</option>`).join('')}` : '<option value="">Geen transacties gevonden</option>';
    form.elements.transactionId.disabled = !transactions.length;
    if (document.querySelector('#liability-relation-form').elements.targetType.value === 'transaction') updateLiabilityRelationForm();
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

function updateLiabilityRelationForm() {
  const form = document.querySelector('#liability-relation-form');
  const targetType = form.elements.targetType.value;
  const items = targetType === 'asset' ? liabilityRelationAssetsState : targetType === 'document' ? liabilityRelationDocumentsState : liabilityRelationTransactionsState;
  form.elements.relationType.innerHTML = targetType === 'asset' ? '<option value="financed_by">Gefinancierd door</option>' : targetType === 'document' ? '<option value="evidence_for">Bewijs voor</option>' : '<option value="relates_to">Heeft betrekking op</option>';
  form.elements.targetId.innerHTML = items.length ? `<option value="">Kies een object</option>${items.map((item) => {
    if (targetType === 'asset') return `<option value="${item.id}">${escapeHtml(item.name)}</option>`;
    if (targetType === 'document') return `<option value="${item.id}">${escapeHtml(item.originalFilename)}</option>`;
    return `<option value="${item.id}">${escapeHtml(item.bookedOn)} &middot; ${escapeHtml(item.description)}</option>`;
  }).join('')}` : '<option value="">Geen objecten beschikbaar</option>';
  form.elements.targetId.disabled = !items.length;
}

function renderLiabilityRelations(relations, liabilityID) {
  if (!relations.length) return '<p>Nog geen relaties vastgelegd.</p>';
  return `<ul>${relations.map((relation) => {
    const liabilityIsSource = relation.sourceType === 'liability' && relation.sourceId === liabilityID;
    const relatedType = liabilityIsSource ? relation.targetType : relation.sourceType;
    const relatedLabel = liabilityIsSource ? relation.targetLabel : relation.sourceLabel;
    const period = relation.validFrom || relation.validTo ? `<small>${escapeHtml(relation.validFrom || '...')} t/m ${escapeHtml(relation.validTo || '...')}</small>` : '';
    return `<li><div><strong>${escapeHtml(relatedLabel)}</strong><small>${escapeHtml(entityTypeLabel(relatedType))} &middot; ${escapeHtml(relationTypeLabel(relation.relationType))}</small>${period}${relation.notes ? `<p>${escapeHtml(relation.notes)}</p>` : ''}</div><button type="button" class="icon-button danger" data-liability-action="delete-relation" data-relation-id="${relation.id}" aria-label="Verwijder relatie">&#10005;</button></li>`;
  }).join('')}</ul>`;
}

async function saveLiabilityRelation(event) {
  event.preventDefault();
  if (!selectedLiabilityState) return;
  const form = event.currentTarget;
  const targetID = Number(form.elements.targetId.value);
  if (!targetID) return;
  const liabilityID = selectedLiabilityState.id;
  const input = { targetType: form.elements.targetType.value, targetId: targetID, relationType: form.elements.relationType.value, validFrom: form.elements.validFrom.value, validTo: form.elements.validTo.value, notes: form.elements.notes.value };
  const response = await fetch(`${apiBaseUrl}/api/v1/liabilities/${liabilityID}/relations`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
  if (!response.ok) return window.alert((await response.json()).error || 'Relatie toevoegen mislukt');
  form.elements.validFrom.value = '';
  form.elements.validTo.value = '';
  form.elements.notes.value = '';
  await loadLiabilityDetail(liabilityID, 'relations');
}

async function deleteLiabilityRelation(relationID) {
  if (!selectedLiabilityState || !window.confirm('Deze relatie verwijderen?')) return;
  const liabilityID = selectedLiabilityState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/liabilities/${liabilityID}/relations/${relationID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Relatie verwijderen mislukt');
  await loadLiabilityDetail(liabilityID, 'relations');
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
  list.innerHTML = '<p>Dossiers laden...</p>';
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/cases?${params}`);
    const cases = await response.json();
    if (!response.ok) throw new Error(cases.error || 'Dossiers laden mislukt');
    if (requestID !== caseListRequestID) return;
    casesState = cases;
    casesLoaded = true;
    list.innerHTML = renderCaseList(cases);
  } catch (error) {
    if (requestID === caseListRequestID) list.innerHTML = `<p class="error">${escapeHtml(error.message)}</p>`;
  }
}

function renderCaseList(cases) {
  if (!cases.length) return '<p>Geen dossiers gevonden.</p>';
  return `<ul>${cases.map((item) => `<li><button type="button" class="asset-list-button case-list-button ${selectedCaseState?.id === item.id ? 'active' : ''}" data-case-id="${item.id}"><span><strong>${escapeHtml(item.title)}</strong><small>${escapeHtml(item.caseType)} &middot; ${escapeHtml(caseStatusLabel(item.status))}</small></span>${item.startDate ? `<time datetime="${escapeHtml(item.startDate)}">${escapeHtml(item.startDate)}</time>` : ''}</button></li>`).join('')}</ul>`;
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
  form.elements.status.value = item?.status || 'active';
  document.querySelector('#case-form-title').textContent = item ? 'Dossier wijzigen' : 'Nieuw dossier';
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
    status: form.elements.status.value,
  };
  try {
    const response = await fetch(id ? `${apiBaseUrl}/api/v1/cases/${id}` : `${apiBaseUrl}/api/v1/cases`, { method: id ? 'PATCH' : 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    const item = await response.json();
    if (!response.ok) throw new Error(item.error || 'Dossier opslaan mislukt');
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
      throw new Error(item.error || documentsResult.error || transactions.error || relations.error || allDocumentsResult.error || assets.error || liabilities.error || eventsResult?.error || 'Dossier laden mislukt');
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

function renderCaseOverview(item) {
  return `<dl class="asset-overview-grid"><div><dt>Type</dt><dd>${escapeHtml(item.caseType)}</dd></div><div><dt>Status</dt><dd>${escapeHtml(caseStatusLabel(item.status))}</dd></div><div><dt>Startdatum</dt><dd>${escapeHtml(item.startDate || 'Niet vastgelegd')}</dd></div><div><dt>Einddatum</dt><dd>${escapeHtml(item.endDate || 'Niet vastgelegd')}</dd></div></dl><p class="asset-description">${escapeHtml(item.description || 'Geen omschrijving.')}</p>`;
}

function switchCaseDetailSection(target) {
  document.querySelectorAll('[data-case-detail-target]').forEach((button) => button.classList.toggle('active', button.dataset.caseDetailTarget === target));
  document.querySelectorAll('[data-case-detail-section]').forEach((section) => section.classList.toggle('active', section.dataset.caseDetailSection === target));
}

async function deleteSelectedCase() {
  if (!selectedCaseState || !window.confirm(`Dossier "${selectedCaseState.title}" verwijderen?`)) return;
  const caseID = selectedCaseState.id;
  const response = await fetch(`${apiBaseUrl}/api/v1/cases/${caseID}`, { method: 'DELETE' });
  if (!response.ok) return window.alert((await response.json()).error || 'Dossier verwijderen mislukt');
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
  return `<ul>${documents.map((item) => `<li><div><a href="${apiBaseUrl}/api/v1/documents/${item.id}/content" target="_blank" rel="noopener">${escapeHtml(item.originalFilename)}</a><small>${escapeHtml(item.documentDate || item.mimeType || '')}${item.sizeBytes == null ? '' : ` &middot; ${formatFileSize(item.sizeBytes)}`}</small>${item.description ? `<span>${escapeHtml(item.description)}</span>` : ''}</div><button type="button" class="icon-button danger" data-case-action="unlink-document" data-document-id="${item.id}" aria-label="Verwijder koppeling">&#10005;</button></li>`).join('')}</ul>`;
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
    case_created: 'Dossier aangemaakt', case_updated: 'Dossier gewijzigd', case_completed: 'Dossier afgerond',
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
    button.classList.toggle('active', button.dataset.configTarget === target);
  });
  document.querySelectorAll('.config-section').forEach((section) => {
    section.classList.toggle('active', section.dataset.configSection === target);
  });
}

function switchMainSection(event) {
  const target = event.currentTarget.dataset.mainTarget;
  activateMainSection(target);
}

function activateMainSection(target) {
  document.querySelectorAll('.main-tab').forEach((button) => {
    button.classList.toggle('active', button.dataset.mainTarget === target);
  });
  document.querySelectorAll('.app-tab-section').forEach((section) => {
    section.classList.toggle('active', section.dataset.mainSection === target);
  });
  if (target === 'assets' && !assetsLoaded) {
    loadAssets();
  }
  if (target === 'liabilities' && !liabilitiesLoaded) {
    loadLiabilities();
  }
  if (target === 'cases' && !casesLoaded) {
    loadCases();
  }
}

async function openRelatedDomainDetail(event) {
  const button = event.target.closest('[data-related-domain-type][data-related-domain-id]');
  if (!button) return;
  const id = Number(button.dataset.relatedDomainId);
  const destination = {
    asset: { section: 'assets', load: loadAssetDetail },
    liability: { section: 'liabilities', load: loadLiabilityDetail },
    case: { section: 'cases', load: loadCaseDetail },
  }[button.dataset.relatedDomainType];
  if (!destination || !Number.isSafeInteger(id) || id <= 0) return;
  activateMainSection(destination.section);
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
  document.querySelector('#main-category-list').innerHTML = renderMainCategoryList();
  renderSubcategoryEditor(id);
}

async function deleteMainCategory(id) {
  const current = mainCategoriesState.find((category) => category.id === id);
  if (!window.confirm(`Hoofdcategorie "${current?.name || ''}" verwijderen? Subcategorieen worden nu ook weggegooid.`)) {
    return;
  }

  const response = await fetch(`${apiBaseUrl}/api/v1/main-categories/${id}`, { method: 'DELETE' });
  if (!response.ok) {
    const data = await response.json();
    window.alert(data.error || 'Hoofdcategorie verwijderen mislukt');
    return;
  }

  mainCategoriesState = mainCategoriesState.filter((category) => category.id !== id);
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
  document.querySelector('[data-summary="accounts"]').textContent = accountsState.length;
  document.querySelector('.dashboard-account-list').innerHTML = renderDashboardAccountList();
  const dashboardSelect = document.querySelector('#dashboard-account-select');
  if (dashboardSelect) {
    dashboardSelect.innerHTML = renderTransactionAccountOptions();
    loadDashboardTransactions();
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
