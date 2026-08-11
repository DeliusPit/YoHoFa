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
    </section>
  `;

  document.querySelector('#main-category-form').addEventListener('submit', addMainCategory);
  document.querySelector('#main-category-list').addEventListener('click', handleMainCategoryAction);
  document.querySelector('#subcategory-editor').addEventListener('click', handleSubcategoryAction);
  document.querySelector('#account-form').addEventListener('submit', saveAccount);
  document.querySelector('#account-list').addEventListener('click', handleAccountAction);
  document.querySelector('#account-form').addEventListener('click', handleAccountAction);
  document.querySelector('#hidden-transactions-result').addEventListener('click', unhideFromTable);
  document.querySelector('#chart-filter-form').addEventListener('change', handleChartFilterChange);
  document.querySelector('#chart-filter-form').addEventListener('click', handleDashboardPeriodClick);
  document.querySelector('#transaction-filter-form').addEventListener('change', handleTransactionFilterChange);
  document.querySelector('#transaction-filter-form').addEventListener('click', handleTransactionPeriodClick);
  document.querySelector('#account-transaction-search').addEventListener('input', resetAndLoadDashboardTransactions);
  document.querySelector('#transaction-result').addEventListener('click', categorizeFromTable);
  document.querySelector('#transaction-result').addEventListener('click', hideFromTable);
  document.querySelector('#dashboard-transaction-result').addEventListener('click', categorizeFromTable);
  document.querySelector('#dashboard-transaction-result').addEventListener('click', hideFromTable);
  document.querySelector('#chart-result').addEventListener('click', handleDashboardChartSelection);
  document.querySelector('.dashboard-account-list').addEventListener('click', handleDashboardAccountSelection);
  document.querySelector('#transaction-result').addEventListener('click', handleAccountPagination);
  document.querySelector('#category-transaction-filter-form select[name="mainCategoryId"]').addEventListener('change', updateCategoryTransactionSubcategories);
  document.querySelector('#category-transaction-filter-form select[name="categoryId"]').addEventListener('change', resetAndLoadDashboardTransactions);
  document.querySelectorAll('.main-tab').forEach((button) => button.addEventListener('click', switchMainSection));
  document.querySelectorAll('.config-tab').forEach((button) => button.addEventListener('click', switchConfigSection));
  document.querySelector('#import-form').addEventListener('submit', previewImport);
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
      <td>${escapeHtml(transaction.description)}</td>
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
      <td>${escapeHtml(transaction.description)}</td>
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
  document.querySelectorAll('.main-tab').forEach((button) => {
    button.classList.toggle('active', button.dataset.mainTarget === target);
  });
  document.querySelectorAll('.app-tab-section').forEach((section) => {
    section.classList.toggle('active', section.dataset.mainSection === target);
  });
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
