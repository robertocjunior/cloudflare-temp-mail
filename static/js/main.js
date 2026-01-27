// --- TAG SYSTEM CLASS (THUNDERBIRD STYLE) ---
class TagSystem {
    constructor(inputId, containerId, suggestionId) {
        this.input = document.getElementById(inputId);
        this.container = document.getElementById(containerId);
        this.suggestionBox = document.getElementById(suggestionId);
        this.tags = [];
        this.existingTags = [];
        this.deleteMarked = false;

        this.init();
    }

    async fetchExistingTags() {
        try {
            const res = await fetch('/api/tags');
            this.existingTags = await res.json();
        } catch (e) { this.existingTags = []; }
    }

    init() {
        this.input.addEventListener('keydown', (e) => {
            // RESET IMEDIATO: Se apertar qualquer coisa que não seja Backspace, desmarca
            if (e.key !== 'Backspace' && this.deleteMarked) {
                this.unmarkDelete();
            }

            // CRIAR TAG: Enter, Tab ou Espaço
            if (e.key === 'Enter' || e.key === 'Tab' || e.key === ' ') {
                if (this.input.value.trim() !== '') {
                    e.preventDefault();
                    this.addTag(this.input.value);
                } else {
                    if (e.key === ' ') e.preventDefault();
                }
            }

            // DELETAR TAG: Lógica precisa
            if (e.key === 'Backspace' && this.input.value === '' && this.tags.length > 0) {
                if (this.deleteMarked) {
                    // 2º Toque: Deleta de verdade
                    e.preventDefault(); // Evita comportamento nativo do navegador
                    this.removeTag(this.tags.length - 1);
                    this.deleteMarked = false;
                } else {
                    // 1º Toque: Seleciona para deletar
                    e.preventDefault(); // IMPORTANTÍSSIMO: Evita que o browser tente apagar algo invisível
                    this.markDelete();
                }
            }
        });

        this.input.addEventListener('input', () => this.showSuggestions());
        this.input.addEventListener('focus', () => this.showSuggestions());

        this.input.addEventListener('blur', () => {
            // Blur = Cancelar seleção de delete
            if (this.deleteMarked) this.unmarkDelete();

            setTimeout(() => {
                if (this.input.value.trim() !== '') {
                    this.addTag(this.input.value);
                }
                this.suggestionBox.style.display = 'none';
            }, 150);
        });

        document.addEventListener('click', (e) => {
            if (!this.input.contains(e.target) && !this.container.contains(e.target) && !this.suggestionBox.contains(e.target)) {
                this.suggestionBox.style.display = 'none';
                // Click fora = Cancelar seleção
                if (this.deleteMarked) this.unmarkDelete();
            }
        });
    }

    // Seleciona visualmente a última tag
    markDelete() {
        const pills = this.container.querySelectorAll('.tag-pill');
        if (pills.length > 0) {
            pills[pills.length - 1].classList.add('mark-delete');
            this.deleteMarked = true;
        }
    }

    // Remove a seleção visual
    unmarkDelete() {
        const pills = this.container.querySelectorAll('.tag-pill');
        pills.forEach(p => p.classList.remove('mark-delete'));
        this.deleteMarked = false;
    }

    addTag(name) {
        const cleanName = name.replace(/,/g, '').trim().toLowerCase();

        if (!cleanName) return;

        if (this.tags.includes(cleanName)) {
            this.input.value = '';
            return;
        }

        this.tags.push(cleanName);
        this.input.value = '';
        this.deleteMarked = false;
        this.render();
        this.suggestionBox.style.display = 'none';
        this.input.focus();
    }

    removeTag(index) {
        this.tags.splice(index, 1);
        this.render();
    }

    reset() {
        this.tags = [];
        this.input.value = '';
        this.deleteMarked = false;
        this.render();
        this.fetchExistingTags();
    }

    getTags() {
        return this.tags;
    }

    render() {
        this.container.innerHTML = '';
        this.tags.forEach((tag, index) => {
            const el = document.createElement('div');
            el.className = 'tag-pill';
            el.innerHTML = `<span>${tag}</span><i class="fa-solid fa-times" onclick="tagSystems['${this.input.id}'].removeTag(${index})"></i>`;
            this.container.appendChild(el);
        });
    }

    showSuggestions() {
        const val = this.input.value.toLowerCase().trim();
        if (!val) {
            this.suggestionBox.style.display = 'none';
            return;
        }

        const matches = this.existingTags.filter(t => t.name.toLowerCase().includes(val) && !this.tags.includes(t.name));

        this.suggestionBox.innerHTML = '';
        if (matches.length === 0) {
            this.suggestionBox.style.display = 'none';
            return;
        }

        matches.forEach(t => {
            const div = document.createElement('div');
            div.className = 'suggestion-item';
            div.innerHTML = `<span class="color-dot" style="background-color:${t.color}"></span> ${t.name}`;
            div.onmousedown = (e) => {
                e.preventDefault();
                this.addTag(t.name);
            };
            this.suggestionBox.appendChild(div);
        });
        this.suggestionBox.style.display = 'block';
    }
}

// Global instances registry
const tagSystems = {};

// Inicializa os sistemas de tag
window.addEventListener('DOMContentLoaded', () => {
    tagSystems['tag-input-create'] = new TagSystem('tag-input-create', 'tags-container-create', 'suggestions-create');
    tagSystems['tag-input-custom'] = new TagSystem('tag-input-custom', 'tags-container-custom', 'suggestions-custom');
});


// --- DROPDOWN LOGIC ---
function toggleCreateDropdown() {
    const dd = document.getElementById('create-dropdown');
    dd.classList.toggle('hidden');
}

window.addEventListener('click', function (e) {
    const area = document.getElementById('create-btn-area');
    if (!area.contains(e.target)) {
        document.getElementById('create-dropdown').classList.add('hidden');
    }
});

// --- INSTANT MODAL OPEN LOGIC ---
async function openCreateModal() {
    document.getElementById('create-dropdown').classList.add('hidden');
    document.getElementById('create-modal').classList.remove('hidden');
    document.getElementById('create-modal-loading').classList.remove('hidden');
    document.getElementById('create-modal-content').classList.add('hidden');

    tagSystems['tag-input-create'].reset(); // Reset tags

    await loadDestinations();
    document.getElementById('create-modal-loading').classList.add('hidden');
    document.getElementById('create-modal-content').classList.remove('hidden');
}

function closeCreateModal() {
    document.getElementById('create-modal').classList.add('hidden');
}

// --- CUSTOM EMAIL LOGIC ---
async function openCustomModal() {
    document.getElementById('create-dropdown').classList.add('hidden');
    document.getElementById('custom-modal').classList.remove('hidden');
    document.getElementById('custom-modal-loading').classList.remove('hidden');
    document.getElementById('custom-modal-content').classList.add('hidden');

    tagSystems['tag-input-custom'].reset(); // Reset tags

    const destsPromise = loadDestinations();
    const configPromise = fetch('/api/config').then(r => r.json()).then(cfg => {
        if (cfg.domain) document.getElementById('domain-suffix').innerText = '@' + cfg.domain;
    }).catch(() => { });

    await Promise.all([destsPromise, configPromise]);

    document.getElementById('custom-modal-loading').classList.add('hidden');
    document.getElementById('custom-modal-content').classList.remove('hidden');
}

function closeCustomModal() {
    document.getElementById('custom-modal').classList.add('hidden');
}

async function confirmCustomEmail() {
    const alias = document.getElementById('custom-alias-input').value.trim();
    const dest = document.getElementById('custom-dest-select').value;
    const domainSuffix = document.getElementById('domain-suffix').innerText;
    const tags = tagSystems['tag-input-custom'].getTags(); // GET TAGS

    if (!alias) { alert("Digite um alias."); return; }

    const fullEmail = alias + domainSuffix;

    const checkRes = await fetch(`/api/check?email=${fullEmail}`);
    const checkData = await checkRes.json();

    if (checkData.exists) {
        closeCustomModal();
        openConfirmModal(
            'Email Já Existe',
            `O endereço ${fullEmail} já está no histórico. Deseja recriá-lo por mais 5 minutos?`,
            () => executeRecreate(fullEmail, dest, tags),
            false
        );
        return;
    }

    const btn = document.getElementById('btn-confirm-custom');
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Criando...';
    btn.disabled = true;

    try {
        const res = await fetch('/api/create', {
            method: 'POST',
            body: JSON.stringify({ email: fullEmail, destination: dest, tags: tags })
        });

        if (res.ok) {
            showToast('Email Personalizado Criado!', 'success');
            closeCustomModal();
            switchTab('dashboard');
        } else {
            const txt = await res.text();
            showToast(txt, 'error');
        }
    } catch (e) {
        showToast('Erro de conexão', 'error');
    } finally {
        btn.innerHTML = '<i class="fa-solid fa-check"></i> Criar Personalizado';
        btn.disabled = false;
    }
}

// --- GENERIC CONFIRMATION ---
let pendingConfirmAction = null;

function openConfirmModal(title, msg, actionCallback, isDestructive = false) {
    document.getElementById('confirm-title').innerText = title;
    document.getElementById('confirm-desc').innerText = msg;
    pendingConfirmAction = actionCallback;

    const btn = document.getElementById('btn-modal-confirm-action');
    if (isDestructive) {
        btn.className = "flex-1 py-2 rounded-lg bg-red-600 hover:bg-red-500 text-white font-bold shadow-lg transition text-sm";
    } else {
        btn.className = "flex-1 py-2 rounded-lg bg-orange-600 hover:bg-orange-500 text-white font-bold shadow-lg transition text-sm";
    }

    document.getElementById('confirm-modal').classList.remove('hidden');
}

function closeConfirmModal() {
    document.getElementById('confirm-modal').classList.add('hidden');
    pendingConfirmAction = null;
}

document.getElementById('btn-modal-confirm-action').addEventListener('click', () => {
    if (pendingConfirmAction) pendingConfirmAction();
    closeConfirmModal();
});

// --- SIDEBAR & TABS ---
let sidebarCollapsed = false;
function toggleSidebar() {
    sidebarCollapsed = !sidebarCollapsed;
    const sidebar = document.getElementById('sidebar');
    const chevron = document.getElementById('chevron');
    if (sidebarCollapsed) {
        sidebar.classList.add('collapsed');
        chevron.style.transform = "rotate(180deg)";
    } else {
        sidebar.classList.remove('collapsed');
        chevron.style.transform = "rotate(0deg)";
    }
}

function switchTab(tab) {
    document.querySelectorAll('.view-section').forEach(el => el.classList.add('hidden'));
    document.getElementById(`view-${tab}`).classList.remove('hidden');
    const titles = { 'dashboard': 'Painel de Controle', 'history': 'Histórico de Emails', 'config': 'Configurações do Sistema' };
    document.getElementById('page-title').innerText = titles[tab];
    const btnArea = document.getElementById('create-btn-area');
    if (tab === 'dashboard') btnArea.style.display = 'block'; else btnArea.style.display = 'none';

    document.getElementById('create-dropdown').classList.add('hidden');

    document.querySelectorAll('aside nav button').forEach(b => {
        b.classList.remove('bg-slate-700', 'text-white');
        b.classList.add('text-slate-400');
    });
    const activeBtn = document.getElementById(`nav-${tab}`);
    activeBtn.classList.add('bg-slate-700', 'text-white');
    activeBtn.classList.remove('text-slate-400');

    if (tab === 'dashboard') loadActive();
    if (tab === 'history') loadHistory();
    if (tab === 'config') { loadConfig(); loadDestinations(); }
}

// --- HELPERS FOR TAG RENDERING ---
function renderTagsHTML(tags) {
    if (!tags || tags.length === 0) return '';
    return tags.map(t =>
        `<span class="tag-badge" style="background-color: ${t.color}">${t.name}</span>`
    ).join('');
}

function getTagsString(tags) {
    if (!tags) return '';
    return tags.map(t => t.name).join(' ');
}

// --- HISTORY ---
async function loadHistory() {
    const res = await fetch('/api/history');
    const list = await res.json();
    const tbody = document.getElementById('history-table');
    tbody.innerHTML = '';

    list.forEach(item => {
        const row = document.createElement('tr');
        row.className = "hover:bg-slate-800/50 transition border-b border-slate-700/50 last:border-0 history-row";

        const statusHtml = item.active
            ? '<span class="inline-flex items-center gap-1 bg-green-500/20 text-green-400 px-2 py-0.5 rounded text-xs border border-green-500/30"><span class="w-1.5 h-1.5 rounded-full bg-green-500"></span>ATIVO</span>'
            : '<span class="inline-flex items-center gap-1 bg-slate-700 text-slate-400 px-2 py-0.5 rounded text-xs">EXPIRADO</span>';

        let actionBtn = '';
        if (!item.active) {
            const tagsList = item.tags ? item.tags.map(t => t.name) : [];
            const tagsJson = JSON.stringify(tagsList).replace(/"/g, '&quot;');
            actionBtn = `
                        <button onclick="confirmRecreate('${item.email}', '${item.destination}', ${tagsJson})" class="text-orange-500 hover:text-white hover:bg-orange-600 px-3 py-1.5 rounded transition text-xs font-bold flex items-center gap-1 ml-auto border border-orange-500/30 hover:border-orange-500">
                            <i class="fa-solid fa-rotate-right"></i> Recriar
                        </button>`;
        }

        // Hidden field for search
        const tagsStr = getTagsString(item.tags);

        row.innerHTML = `
                    <td class="p-4 font-mono text-white select-all alias-cell">${item.email}</td>
                    <td class="p-4"><div class="flex flex-wrap max-w-[200px]">${renderTagsHTML(item.tags)}</div><span class="hidden tags-search-val">${tagsStr}</span></td>
                    <td class="p-4 text-slate-400 text-xs dest-cell">${item.destination}</td>
                    <td class="p-4 text-slate-500">${new Date(item.created_at).toLocaleString()}</td>
                    <td class="p-4 text-center">${statusHtml}</td>
                    <td class="p-4 text-right">${actionBtn}</td>
                `;
        tbody.appendChild(row);
    });
}

function filterHistory() {
    const term = document.getElementById('history-search').value.toLowerCase();
    const rows = document.querySelectorAll('.history-row');
    rows.forEach(row => {
        const alias = row.querySelector('.alias-cell').innerText.toLowerCase();
        const dest = row.querySelector('.dest-cell').innerText.toLowerCase();
        const tags = row.querySelector('.tags-search-val').innerText.toLowerCase();
        if (alias.includes(term) || dest.includes(term) || tags.includes(term)) {
            row.style.display = '';
        } else {
            row.style.display = 'none';
        }
    });
}

function confirmRecreate(email, destination, tags) {
    openConfirmModal(
        'Recriar Email',
        `Deseja reativar o endereço ${email} por mais 5 minutos?`,
        () => executeRecreate(email, destination, tags),
        false
    );
}

async function executeRecreate(email, destination, tags) {
    showToast('Recriando...', 'success');
    try {
        const res = await fetch('/api/create', {
            method: 'POST',
            body: JSON.stringify({ email: email, destination: destination, tags: tags })
        });

        if (res.ok) {
            showToast('Email Recriado!', 'success');
            switchTab('dashboard');
        } else {
            const txt = await res.text();
            showToast(txt, 'error');
        }
    } catch (e) {
        showToast('Erro de conexão', 'error');
    }
}

// --- DELETIONS ---
function confirmDeleteEmail(id) {
    openConfirmModal(
        'Destruir Email',
        'Isso removerá o redirecionamento imediatamente. Tem certeza?',
        () => executeDeleteEmail(id),
        true
    );
}

async function executeDeleteEmail(id) {
    await fetch(`/api/delete?id=${id}`, { method: 'DELETE' });
    loadActive();
    showToast('Email destruído.', 'success');
}

function confirmDeleteDest(id) {
    openConfirmModal(
        'Remover Destino',
        'Tem certeza que deseja remover este email da sua conta Cloudflare?',
        () => executeDeleteDest(id),
        true
    );
}

async function executeDeleteDest(id) {
    try {
        const res = await fetch(`/api/destinations?id=${id}`, { method: 'DELETE' });
        if (res.ok) {
            showToast('Destino removido.', 'success');
            loadDestinations();
        } else {
            showToast('Erro ao remover.', 'error');
        }
    } catch (e) { showToast('Erro de conexão', 'error'); }
}

// --- TOGGLE PIN LOGIC ---
function confirmPin(id, isCurrentlyPinned) {
    const action = isCurrentlyPinned ? "Desafixar" : "Fixar";
    const msg = isCurrentlyPinned
        ? "Ao desafixar, o email expirará em 5 minutos. Deseja continuar?"
        : "Ao fixar, o email NÃO expirará automaticamente. Deseja continuar?";

    openConfirmModal(
        `${action} Email`,
        msg,
        () => executePin(id, !isCurrentlyPinned),
        false // Not typically dangerous
    );
}

async function executePin(id, newState) {
    try {
        const res = await fetch('/api/pin', {
            method: 'POST',
            body: JSON.stringify({ id: id, pinned: newState })
        });

        if (res.ok) {
            showToast(newState ? 'Email Fixado!' : 'Email Desafixado (5 min restantes)', 'success');
            loadActive();
        } else {
            showToast('Erro ao alterar status', 'error');
        }
    } catch (e) {
        showToast('Erro de conexão', 'error');
    }
}

// --- STANDARD LOGIC ---
async function loadDestinations() {
    const container = document.getElementById('dest-list');
    if (document.getElementById('view-config').classList.contains('hidden') === false) {
        container.innerHTML = '<div class="text-slate-500 text-sm animate-pulse">Sincronizando...</div>';
    }

    try {
        const res = await fetch('/api/destinations');
        if (!res.ok) {
            const err = await res.text();
            container.innerHTML = `<div class="text-red-400 text-sm bg-red-900/20 p-3 rounded border border-red-900/50"><i class="fa-solid fa-circle-exclamation mr-2"></i>${err}</div>`;
            return;
        }

        const list = await res.json();

        container.innerHTML = '';
        if (!list || list.length === 0) {
            container.innerHTML = '<p class="text-slate-500 italic text-sm">Nenhum email encontrado.</p>';
        } else {
            list.forEach(d => {
                const isVerified = !!d.verified;
                const statusBadge = isVerified
                    ? `<span class="text-xs bg-green-500/10 text-green-400 border border-green-500/20 px-2 py-0.5 rounded font-bold uppercase tracking-wider">Verificado</span>`
                    : `<span class="text-xs bg-yellow-500/10 text-yellow-400 border border-yellow-500/20 px-2 py-0.5 rounded font-bold uppercase tracking-wider">Pendente</span>`;
                const iconClass = isVerified ? "fa-check-circle text-green-500" : "fa-clock text-yellow-500";
                const opacityClass = isVerified ? "" : "opacity-75";

                const item = document.createElement('div');
                item.className = 'flex items-center justify-between bg-slate-900 p-3 rounded border border-slate-700 transition hover:border-slate-600';
                item.innerHTML = `
                            <div class="flex items-center gap-3 ${opacityClass}">
                                <i class="fa-solid ${iconClass}"></i>
                                <div>
                                    <div class="text-slate-200 font-medium">${d.email}</div>
                                    <div class="text-xs text-slate-500">${statusBadge}</div>
                                </div>
                            </div>
                            <button onclick="confirmDeleteDest('${d.tag}')" class="text-slate-600 hover:text-red-500 px-3 py-2 transition rounded hover:bg-red-500/10">
                                <i class="fa-solid fa-trash"></i>
                            </button>
                        `;
                container.appendChild(item);
            });
        }

        const selects = document.querySelectorAll('.dest-select-target');
        selects.forEach(sel => {
            sel.innerHTML = '';
            list.forEach(d => {
                const isVerified = !!d.verified;
                const opt = document.createElement('option');
                opt.value = d.email;
                opt.innerText = isVerified ? d.email : `${d.email} (Pendente)`;
                if (!isVerified) opt.disabled = true;
                sel.appendChild(opt);
            });
        });

    } catch (e) {
        container.innerHTML = '<div class="text-red-400 text-sm">Erro de conexão.</div>';
    }
}

function openAddDestModal() { document.getElementById('add-dest-modal').classList.remove('hidden'); }
function closeAddDestModal() { document.getElementById('add-dest-modal').classList.add('hidden'); }

async function confirmAddDest() {
    const input = document.getElementById('new-dest-input');
    const email = input.value;
    const btn = document.getElementById('btn-confirm-add');

    if (!email) return;

    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> Processando...';
    btn.disabled = true;

    try {
        const res = await fetch('/api/destinations', {
            method: 'POST',
            body: JSON.stringify({ email: email })
        });

        if (res.ok) {
            showToast('Email adicionado! Verifique sua caixa de entrada.', 'success');
            input.value = '';
            closeAddDestModal();
            loadDestinations();
        } else {
            const txt = await res.text();
            showToast(txt, 'error');
        }
    } catch (e) {
        showToast('Erro de conexão', 'error');
    } finally {
        btn.innerHTML = '<i class="fa-solid fa-paper-plane"></i> Enviar Solicitação';
        btn.disabled = false;
    }
}

async function loadConfig() {
    try {
        const res = await fetch('/api/config');
        const data = await res.json();
        if (data.cf_token) {
            document.getElementById('cfg-token').value = data.cf_token;
            document.getElementById('cfg-zone').value = data.zone_id;
            document.getElementById('cfg-domain').value = data.domain;
        }
    } catch (e) { }
}

async function saveConfig(e) {
    e.preventDefault();
    const data = {
        cf_token: document.getElementById('cfg-token').value,
        zone_id: document.getElementById('cfg-zone').value,
        domain: document.getElementById('cfg-domain').value
    };
    const res = await fetch('/api/config', { method: 'POST', body: JSON.stringify(data) });
    if (res.ok) {
        showToast('Credenciais Salvas!', 'success');
        loadConfig();
        loadDestinations();
    }
}

async function confirmCreateEmail() {
    const dest = document.getElementById('modal-dest-select').value;
    const tags = tagSystems['tag-input-create'].getTags(); // GET TAGS

    if (!dest) { alert("Selecione um destino válido."); return; }
    const btn = document.getElementById('btn-confirm-create');
    btn.innerHTML = '<i class="fa-solid fa-circle-notch fa-spin"></i> Criando...';
    btn.disabled = true;

    try {
        const res = await fetch('/api/create', {
            method: 'POST',
            body: JSON.stringify({ destination: dest, tags: tags })
        });

        if (res.ok) {
            closeCreateModal();
            loadActive();
            showToast('Email Criado com Sucesso!', 'success');
        } else {
            const txt = await res.text();
            showToast(txt, 'error');
        }
    } catch (e) {
        showToast('Erro de conexão', 'error');
    } finally {
        btn.innerHTML = '<i class="fa-solid fa-magic-wand-sparkles"></i> Gerar Agora';
        btn.disabled = false;
    }
}

async function loadActive() {
    const res = await fetch('/api/active');
    const list = await res.json();
    const grid = document.getElementById('active-grid');
    grid.innerHTML = '';
    if (!list || list.length === 0) {
        document.getElementById('empty-dashboard').classList.remove('hidden');
        return;
    }
    document.getElementById('empty-dashboard').classList.add('hidden');
    list.forEach(item => {
        const created = new Date(item.created_at);
        const expires = new Date(created.getTime() + 5 * 60000);

        // Pin logic styling
        const isPinned = item.pinned;
        const borderClass = isPinned ? 'border-green-500/50' : 'border-slate-700';
        const pinBtnColor = isPinned ? 'text-green-400 bg-green-500/10 border-green-500/30' : 'text-slate-400 hover:text-white hover:bg-slate-700';
        const timerDisplay = isPinned ? '<i class="fa-solid fa-infinity"></i>' : '--:--';
        const progressWidth = isPinned ? '100%' : '0%';
        const progressColor = isPinned ? 'bg-green-500' : 'bg-gradient-to-r from-orange-500 to-red-500';

        const card = document.createElement('div');
        card.className = `bg-slate-800 border ${borderClass} rounded-xl p-5 relative overflow-hidden group hover:border-orange-500/50 transition shadow-lg`;

        // Tags HTML
        const tagsHtml = renderTagsHTML(item.tags);
        const tagsContainer = tagsHtml ? `<div class="mb-3 flex flex-wrap">${tagsHtml}</div>` : '';

        card.innerHTML = `
                    <div class="absolute top-0 left-0 h-1 ${progressColor} w-full transition-all duration-1000" id="prog-${item.id}" style="width: ${progressWidth}"></div>
                    
                    <div class="flex justify-between items-start mb-2">
                        <div class="text-xs text-slate-400 font-mono flex items-center gap-1 max-w-[65%] truncate" title="${item.destination}">
                            <i class="fa-solid fa-arrow-right-long text-slate-600"></i> ${item.destination}
                        </div>
                        <span id="timer-${item.id}" class="font-mono font-bold text-white bg-slate-900 px-2 py-1 rounded text-sm">${timerDisplay}</span>
                    </div>

                    ${tagsContainer}

                    <div class="mb-5 text-center">
                        <code class="text-lg text-white font-bold cursor-pointer hover:text-orange-400 transition break-all select-all" onclick="copyText('${item.email}')">
                            ${item.email}
                        </code>
                        <p class="text-xs text-slate-500 mt-1">Clique para copiar</p>
                    </div>

                    <div class="flex gap-2">
                        <button onclick="confirmPin('${item.id}', ${isPinned})" class="flex-1 ${pinBtnColor} border border-transparent py-2 rounded-lg font-bold text-sm transition flex items-center justify-center gap-2" title="${isPinned ? 'Desafixar' : 'Fixar para não expirar'}">
                            <i class="fa-solid fa-thumbtack ${isPinned ? '' : 'rotate-45'}"></i>
                        </button>
                        <button onclick="confirmDeleteEmail('${item.id}')" class="flex-[3] bg-slate-700 hover:bg-red-600 text-slate-300 hover:text-white py-2 rounded-lg font-bold text-sm transition flex items-center justify-center gap-2">
                            <i class="fa-solid fa-trash"></i> Destruir
                        </button>
                    </div>
                `;
        grid.appendChild(card);

        if (!isPinned) {
            startLocalTimer(item.id, expires);
        }
    });
}

function startLocalTimer(id, expires) {
    const el = document.getElementById(`timer-${id}`);
    const prog = document.getElementById(`prog-${id}`);
    if (!el) return;
    if (window[`timer_${id}`]) clearInterval(window[`timer_${id}`]);
    window[`timer_${id}`] = setInterval(() => {
        const now = new Date();
        const diff = Math.floor((expires - now) / 1000);
        const total = 300;
        if (diff <= 0) {
            clearInterval(window[`timer_${id}`]);
            el.innerText = "00:00";
            prog.style.width = "0%";
            loadActive();
        } else {
            const m = Math.floor(diff / 60);
            const s = diff % 60;
            el.innerText = `${m}:${s.toString().padStart(2, '0')}`;
            prog.style.width = `${(diff / total) * 100}%`;
        }
    }, 1000);
}

function copyText(txt) {
    navigator.clipboard.writeText(txt);
    showToast('Endereço copiado!', 'success');
}

function showToast(msg, type) {
    const c = document.getElementById('toast-container');
    const t = document.createElement('div');
    const color = type === 'success' ? 'bg-green-600' : 'bg-red-600';
    t.className = `${color} text-white px-4 py-3 rounded shadow-lg pointer-events-auto flex items-center gap-3 transform transition-all translate-x-0`;
    t.innerHTML = `<i class="fa-solid ${type === 'success' ? 'fa-check' : 'fa-triangle-exclamation'}"></i> <span>${msg}</span>`;
    c.appendChild(t);
    setTimeout(() => {
        t.style.opacity = '0';
        t.style.transform = 'translateY(10px)';
        setTimeout(() => t.remove(), 300);
    }, 3000);
}

switchTab('dashboard');