import './style.css';

const { GetConversations, GetMessages, CreateConversation, DeleteConversation, SendMessage, GetSettings, SaveSettings, TestConnection } = window.go.main.App;

let currentConversationID = null;

async function refreshConversations() {
    const conversations = await GetConversations();
    renderConversationList(conversations);
}

function renderConversationList(conversations) {
    const list = document.getElementById('conversation-list');
    if (!conversations || conversations.length === 0) {
        list.innerHTML = '<div style="padding:16px;color:#888;text-align:center">No conversations</div>';
        return;
    }
    list.innerHTML = conversations.map(conv => `
        <div class="conversation-item ${currentConversationID === conv.ID ? 'active' : ''}" onclick="selectConversation(${conv.ID})">
            <div class="conversation-title">${conv.Title || 'New Chat'}</div>
            <button class="delete-btn" onclick="event.stopPropagation(); deleteConversation(${conv.ID})">×</button>
        </div>
    `).join('');
}

async function selectConversation(id) {
    currentConversationID = id;
    await refreshConversations();
    const messages = await GetMessages(id);
    renderMessages(messages);
}

async function loadInitialState() {
    const conversations = await GetConversations();
    if (conversations.length > 0) {
        await selectConversation(conversations[0].ID);
    } else {
        currentConversationID = null;
        renderConversationList(conversations);
        document.getElementById('messages').innerHTML = '';
    }
}

function renderMessages(messages) {
    const container = document.getElementById('messages');
    if (!messages) messages = [];
    container.innerHTML = messages.map(msg => `
        <div class="message ${msg.Role}">
            <div class="message-content">${escapeHtml(msg.Content)}</div>
        </div>
    `).join('');
    container.scrollTop = container.scrollHeight;
}

function renderSettings() {
    GetSettings().then(settings => {
        document.getElementById('endpoint').value = settings.azure_endpoint || '';
        document.getElementById('apikey').value = settings.azure_api_key || '';
        document.getElementById('deployment').value = settings.azure_deployment || '';
    });
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

window.selectConversation = selectConversation;

window.createNewConversation = async function() {
    currentConversationID = null;
    document.getElementById('messages').innerHTML = '';
    const conversations = await GetConversations();
    renderConversationList(conversations);
};

window.deleteConversation = async function(id) {
    await DeleteConversation(id);
    currentConversationID = null;
    document.getElementById('messages').innerHTML = '';
    
    // Always re-fetch from backend after delete
    const conversations = await GetConversations();
    currentConversationID = null;
    renderConversationList(conversations);
};

window.handleSend = async function() {
    const input = document.getElementById('message-input');
    const content = input.value.trim();
    if (!content) return;
    
    // Create conversation if none exists
    if (!currentConversationID) {
        const conv = await CreateConversation();
        currentConversationID = conv.ID;
        // Show new conversation in sidebar immediately
        const conversations = await GetConversations();
        renderConversationList(conversations);
    }

    // Show user message immediately in UI
    const messagesContainer = document.getElementById('messages');
    const userMsgDiv = document.createElement('div');
    userMsgDiv.className = 'message user';
    userMsgDiv.innerHTML = '<div class="message-content">' + escapeHtml(content) + '</div>';
    messagesContainer.appendChild(userMsgDiv);
    messagesContainer.scrollTop = messagesContainer.scrollHeight;

    input.value = '';
    input.disabled = true;

    try {
        const response = await SendMessage(currentConversationID, content);
        
        if (response && response.startsWith('Error:')) {
            alert(response);
        }
        
        // Always refresh from backend
        await refreshConversations();
        const messages = await GetMessages(currentConversationID);
        renderMessages(messages);
    } catch (err) {
        console.error('Error:', err);
        alert('Error: ' + err);
    } finally {
        input.disabled = false;
        input.focus();
    }
};

window.openSettings = function() {
    document.getElementById('settings-modal').style.display = 'flex';
};

window.closeSettingsModal = function() {
    document.getElementById('settings-modal').style.display = 'none';
};

window.saveSettings = async function() {
    const endpoint = document.getElementById('endpoint').value;
    const apikey = document.getElementById('apikey').value;
    const deployment = document.getElementById('deployment').value;
    
    try {
        await SaveSettings(endpoint, apikey, deployment);
        closeSettingsModal();
        alert('Settings saved!');
    } catch (err) {
        alert('Error saving settings: ' + err);
    }
};

window.testConnection = async function() {
    const endpoint = document.getElementById('endpoint').value;
    const apikey = document.getElementById('apikey').value;
    const deployment = document.getElementById('deployment').value;
    
    try {
        const result = await TestConnection(endpoint, apikey, deployment);
        alert(result);
    } catch (err) {
        alert('Error: ' + err);
    }
};

window.handleKeyPress = function(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        window.handleSend();
    }
};

document.getElementById('message-input').addEventListener('keydown', window.handleKeyPress);

// Initialize
loadInitialState();
renderSettings();