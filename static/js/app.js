/**
 * SISTEMA DE TAGS (ESTILO THUNDERBIRD)
 */
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
        } catch(e) { 
            this.existingTags = []; 
        }
    }

    init() {
        this.input.addEventListener('keydown', (e) => {
            if (e.key !== 'Backspace' && this.deleteMarked) {
                this.unmarkDelete();
            }

            if (e.key === 'Enter' || e.key === 'Tab' || e.key === ' ') {
                if (this.input.value.trim() !== '') {
                    e.preventDefault(); 
                    this.addTag(this.input.value);
                } else {
                    if (e.key === ' ') e.preventDefault();
                }
            }
            
            if (e.key === 'Backspace' && this.input.value === '' && this.tags.length > 0) {
                if (this.deleteMarked) {
                    e.preventDefault();
                    this.removeTag(this.tags.length - 1);
                    this.deleteMarked = false;
                } else {
                    e.preventDefault();
                    this.markDelete();
                }
            }
        });

        this.input.addEventListener('input', () => this.showSuggestions());
        this.input.addEventListener('focus', () => this.showSuggestions());

        this.input.addEventListener('blur', () => {
            if(this.deleteMarked) this.unmarkDelete();
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
                if(this.deleteMarked) this.unmarkDelete();
            }
        });
    }

    markDelete() {
        const pills = this.container.querySelectorAll('.tag-pill');
        if (pills.length > 0) {
            pills[pills.length - 1].classList.add('mark-delete');
            this.deleteMarked = true;
        }
    }

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
        if(!val) {
            this.suggestionBox.style.display = 'none';
            return;
        }
        const matches = this.existingTags.filter(t => t.name.toLowerCase().includes(val) && !this.tags.includes(t.name));
        this.suggestionBox.innerHTML = '';
        if(matches.length === 0) {
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

// Instâncias Globais
const tagSystems = {};

window.addEventListener('DOMContentLoaded', () => {
    tagSystems['tag-input-create'] = new TagSystem('tag-input-create', 'tags-container-create', 'suggestions-create');
    tagSystems['tag-input-custom'] = new TagSystem('tag-input-custom', 'tags-container-custom', 'suggestions-custom');
});

/**
 * LÓGICA DE INTERFACE E API
 */
function toggleCreateDropdown() {
    const dd = document.getElementById('create-dropdown');
    dd.classList.toggle('hidden');
}

window.addEventListener('click', function(e) {
    const area = document.getElementById('create-btn-area');
    if (area && !area.contains(e.target)) {
        document.getElementById('create-dropdown').classList.add('hidden');
    }
});

async function openCreateModal() {
    document.getElementById('create-dropdown').classList.add('hidden');
    document.getElementById('create-modal').classList.remove('hidden');
    document.getElementById('create-modal-loading').classList.remove('hidden');
    document.getElementById('create-modal-content').classList.add('hidden');
    tagSystems['tag-input-create'].reset();
    await loadDestinations(); 
    document.getElementById('create-modal-loading').classList.add('hidden');
    document.getElementById('create-modal-content').classList.remove('hidden');
}

function closeCreateModal() {
    document.getElementById('create-modal').classList.add('hidden');
}

async function openCustomModal() {
    document.getElementById('create-dropdown').classList.add('hidden');
    document.getElementById('custom-modal').classList.remove('hidden');
    document.getElementById('custom-modal-loading').classList.remove('hidden');
    document.getElementById('custom-modal-content').classList.add('hidden');
    tagSystems['tag-input-custom'].reset();
    const destsPromise = loadDestinations();
    const configPromise = fetch('/api/config').then(r => r.json()).then(cfg => {
        if(cfg.domain) document.getElementById('domain-suffix').innerText = '@' + cfg.domain;
    }).catch(()=>{});
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
    const tags = tagSystems['tag-input-custom'].getTags();

    if(!alias) { alert("Digite um alias."); return; }
    const fullEmail = alias + domainSuffix;
    const checkRes = await fetch(`/api/check?email=${fullEmail}`);
    const checkData = await checkRes.json();

    if(checkData.exists) {
        closeCustomModal();
        openConfirmModal('Email Já Existe', `O endereço ${fullEmail} já está no histórico. Deseja recriá-lo?`, () => executeRecreate(fullEmail, dest, tags), false);
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
        if(res.ok) {
            showToast('Email Personalizado Criado!', 'success');
            closeCustomModal();
            switchTab('dashboard');
        } else {
            const txt = await res.text();
            showToast(txt, 'error');
        }
    } catch(e) {
        showToast('Erro de conexão', 'error');
    } finally {
        btn.innerHTML = '<i class="fa-solid fa-check"></i> Criar Personalizado';
        btn.disabled = false;
    }
}

let pendingConfirmAction = null;
function openConfirmModal(title, msg, actionCallback, isDestructive = false) {
    document.getElementById('confirm-title').innerText = title;
    document.getElementById('confirm-desc').innerText = msg;
    pendingConfirmAction = actionCallback;
    const btn = document.getElementById('btn-modal-confirm-action');
    btn.className = isDestructive 
        ? "flex-1 py-2 rounded-lg bg-red-600 hover:bg-red-500 text-white font-bold shadow-lg transition text-sm"
        : "flex-1 py-2 rounded-lg bg-orange-600 hover:bg-orange-500 text-white font-bold shadow-lg transition text-sm";
    document.getElementById('confirm-modal').classList.remove('hidden');
}

function closeConfirmModal() {
    document.getElementById('confirm-modal').classList.add('hidden');
    pendingConfirmAction = null;
}

document.getElementById('btn-modal-confirm-action').addEventListener('click', () => {
    if(pendingConfirmAction) pendingConfirmAction();
    closeConfirmModal();
});

let sidebarCollapsed = false;
function toggleSidebar() {
    sidebarCollapsed = !sidebarCollapsed;
    const sidebar = document.getElementById('sidebar');
    const chevron = document.getElementById('chevron');
    if(sidebarCollapsed) {
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
    if(tab === 'dashboard') btnArea.style.display = 'block'; else btnArea.style.display = 'none';
    document.getElementById('create-dropdown').classList.add('hidden');

    document.querySelectorAll('aside nav button').forEach(b => {
        b.classList.remove('bg-slate-700', 'text-white');
        b.classList.add('text-slate-400');
    });
    const activeBtn = document.getElementById(`nav-${tab}`);
    activeBtn.classList.add('bg-slate-700', 'text-white');
    activeBtn.classList.remove('text-slate-400');

    if(tab === 'dashboard') loadActive();
    if(tab === 'history') loadHistory();
    if(tab === 'config') { loadConfig(); loadDestinations(); }
}

function renderTagsHTML(tags) {
    if (!tags || tags.length === 0) return '';
    return tags.map(t => `<span class="tag-badge" style="background-color: ${t.color}">${t.name}</span>`).join('');
}

async function loadHistory() {
    const res = await fetch('/api/history');
    const list = await res.json();
    const tbody = document.getElementById('history-table');
    tbody.innerHTML = '';
    list.forEach(item => {
        const row = document.createElement('tr');
        row.className = "hover:bg-slate-800/50 transition border-b border-slate-700/50 last:border-0 history-row";
        const statusHtml = item.active 
            ? '<span class="inline-flex items-center gap-1 bg-green-500/20 text-green-400 px-2 py-0.5 rounded text-xs border border-green-500/30">ATIVO</span>'
            : '<span class="inline-flex items-center gap-1 bg-slate-700 text-slate-400 px-2 py-0.5 rounded text-xs">EXPIRADO</span>';

        let actionBtn = item.active ? '' : `<button onclick="confirmRecreate('${item.email}', '${item.destination}', ${JSON.stringify(item.tags ? item.tags.map(t=>t.name) : []).replace(/"/g, '&quot;')})" class="text-orange-500 hover:text-white px-3 py-1.5 rounded transition text-xs border border-orange-500/30">Recriar</button>`;

        row.innerHTML = `
            <td class="p-4 font-mono text-white alias-cell">${item.email}</td>
            <td class="p-4"><div class="flex flex-wrap">${renderTagsHTML(item.tags)}</div></td>
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
    document.querySelectorAll('.history-row').forEach(row => {
        const text = row.innerText.toLowerCase();
        row.style.display = text.includes(term) ? '' : 'none';
    });
}

async function executeRecreate(email, destination, tags) {
    showToast('Recriando...', 'success');
    try {
        const res = await fetch('/api/create', { method: 'POST', body: JSON.stringify({ email, destination, tags }) });
        if(res.ok) { showToast('Email Recriado!', 'success'); switchTab('dashboard'); }
    } catch(e) { showToast('Erro de conexão', 'error'); }
}

async function executeDeleteEmail(id) {
    await fetch(`/api/delete?id=${id}`, { method: 'DELETE' });
    loadActive();
    showToast('Email destruído.', 'success');
}

async function executePin(id, newState) {
    try {
        const res = await fetch('/api/pin', { method: 'POST', body: JSON.stringify({ id, pinned: newState }) });
        if(res.ok) { showToast(newState ? 'Email Fixado!' : 'Email Desafixado', 'success'); loadActive(); }
    } catch(e) { showToast('Erro de conexão', 'error'); }
}

async function loadDestinations() {
    const container = document.getElementById('dest-list');
    try {
        const res = await fetch('/api/destinations');
        const list = await res.json();
        container.innerHTML = '';
        list.forEach(d => {
            const item = document.createElement('div');
            item.className = 'flex items-center justify-between bg-slate-900 p-3 rounded border border-slate-700';
            item.innerHTML = `<span>${d.email}</span><button onclick="confirmDeleteDest('${d.tag}')" class="text-red-500"><i class="fa-solid fa-trash"></i></button>`;
            container.appendChild(item);
        });
        document.querySelectorAll('.dest-select-target').forEach(sel => {
            sel.innerHTML = list.map(d => `<option value="${d.email}">${d.email}${d.verified?'':' (Pendente)'}</option>`).join('');
        });
    } catch(e) {}
}

async function loadActive() {
    const res = await fetch('/api/active');
    const list = await res.json();
    const grid = document.getElementById('active-grid');
    grid.innerHTML = '';
    if(!list || list.length === 0) { document.getElementById('empty-dashboard').classList.remove('hidden'); return; }
    document.getElementById('empty-dashboard').classList.add('hidden');
    list.forEach(item => {
        const card = document.createElement('div');
        card.className = `bg-slate-800 border ${item.pinned ? 'border-green-500/50' : 'border-slate-700'} rounded-xl p-5 relative shadow-lg`;
        card.innerHTML = `
            <div class="mb-3 text-xs text-slate-400 truncate">${item.destination}</div>
            <div class="mb-3">${renderTagsHTML(item.tags)}</div>
            <div class="mb-5 text-center font-bold text-white select-all" onclick="copyText('${item.email}')">${item.email}</div>
            <div class="flex gap-2">
                <button onclick="executePin('${item.id}', ${!item.pinned})" class="flex-1 py-2 rounded bg-slate-700"><i class="fa-solid fa-thumbtack ${item.pinned ? '' : 'rotate-45'}"></i></button>
                <button onclick="executeDeleteEmail('${item.id}')" class="flex-[3] py-2 rounded bg-red-600">Destruir</button>
            </div>
        `;
        grid.appendChild(card);
    });
}

function copyText(txt) {
    navigator.clipboard.writeText(txt);
    showToast('Endereço copiado!', 'success');
}

function showToast(msg, type) {
    const c = document.getElementById('toast-container');
    const t = document.createElement('div');
    t.className = `${type === 'success' ? 'bg-green-600' : 'bg-red-600'} text-white px-4 py-3 rounded shadow-lg flex items-center gap-3`;
    t.innerHTML = `<span>${msg}</span>`;
    c.appendChild(t);
    setTimeout(() => t.remove(), 3000);
}

// Início
switchTab('dashboard');