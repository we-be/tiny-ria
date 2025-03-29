// Main application script
document.addEventListener('DOMContentLoaded', () => {
    // State management
    const state = {
        websocket: null,
        messageHistory: [],
        // Default to dark theme for terminal-like experience
        theme: localStorage.getItem('theme') || 'dark',
        monitoredSymbols: JSON.parse(localStorage.getItem('monitoredSymbols') || '[]'),
        marketIndices: [],
        currentChart: null,
        isPanelCollapsed: localStorage.getItem('isPanelCollapsed') === 'true' || false,
        isTypingEffect: true, // Enable typing effect for messages
    };

    // DOM references
    const elements = {
        chatMessages: document.getElementById('chat-messages'),
        chatForm: document.getElementById('chat-form'),
        chatInput: document.getElementById('chat-input'),
        clearChat: document.getElementById('clear-chat'),
        toggleTheme: document.getElementById('toggle-theme'),
        symbolInput: document.getElementById('symbol-input'),
        addSymbolBtn: document.getElementById('add-symbol-btn'),
        monitoredList: document.getElementById('monitored-list'),
        marketIndices: document.getElementById('market-indices'),
        quickActionBtns: document.querySelectorAll('.quick-action-btn'),
        collapsePanel: document.getElementById('collapse-panel'),
        dataPanel: document.querySelector('.data-panel'),
        priceChart: document.getElementById('price-chart'),
        dataCards: document.querySelector('.data-cards'),
        dataTables: document.querySelector('.data-tables'),
    };

    // Templates
    const templates = {
        message: document.getElementById('message-template'),
        marketIndex: document.getElementById('market-index-template'),
        monitoredSymbol: document.getElementById('monitored-symbol-template'),
        dataCard: document.getElementById('data-card-template'),
        alert: document.getElementById('alert-template'),
    };

    // Initialize
    function init() {
        setupWebSocket();
        setupEventListeners();
        applyTheme();
        updateDataPanel();
        
        // Add a blinking cursor to the chat input
        addBlinkingCursor();

        // Auto-resize textarea as content grows
        autoResizeTextarea(elements.chatInput);

        // Check if panel should be collapsed
        if (state.isPanelCollapsed) {
            elements.dataPanel.classList.add('collapsed');
        }
        
        // Delay the initial data loading to ensure WebSocket is ready
        setTimeout(() => {
            // Only update monitored symbols if WebSocket is ready
            if (state.websocket && state.websocket.readyState === WebSocket.OPEN) {
                updateMonitoredSymbols();
                fetchMarketIndices();
            } else {
                console.log("WebSocket not ready, will retry data loading on open");
            }
        }, 1000);
    }

    // Simple static cursor
    function addBlinkingCursor() {
        // Function simplified - no cursor animation for cleaner design
    }

    // Set up WebSocket connection
    function setupWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        state.websocket = new WebSocket(wsUrl);

        state.websocket.onopen = () => {
            console.log('WebSocket connection established');
            addSystemMessage('Connection established. Welcome to the terminal.');
            
            // Retry data loading once connection is established
            if (state.monitoredSymbols.length > 0) {
                console.log("WebSocket connected, loading data for monitored symbols");
                updateMonitoredSymbols();
            }
            
            // Fetch market indices
            console.log("WebSocket connected, loading market indices");
            fetchMarketIndices();
        };

        state.websocket.onmessage = (event) => {
            const message = JSON.parse(event.data);
            handleIncomingMessage(message);
        };

        state.websocket.onclose = () => {
            console.log('WebSocket connection closed');
            addSystemMessage('> CONNECTION LOST. Attempting to reconnect...');
            
            // Try to reconnect after a delay
            setTimeout(setupWebSocket, 3000);
        };

        state.websocket.onerror = (error) => {
            console.error('WebSocket error:', error);
            addErrorMessage('> ERROR: Connection failed. Please check your connection.');
        };
    }

    // Handle incoming WebSocket messages
    function handleIncomingMessage(message) {
        switch (message.type) {
            case 'system':
                addSystemMessage(message.content);
                break;
            case 'tool_use':
                // Show tool usage as a muted collapsible message
                addToolMessage(message.content);
                break;
            case 'assistant':
                addAssistantMessage(message.content);
                
                // Check for financial data in the message and visualize it
                visualizeFinancialData(message.content);
                break;
            case 'error':
                addErrorMessage(message.content);
                break;
            case 'alert':
                handleAlert(message.data);
                break;
            case 'typing':
                handleTypingIndicator(message.data.status);
                break;
            case 'price_data':
                handlePriceData(message.data);
                break;
            case 'index_data':
                handleIndexData(message.data);
                break;
            default:
                console.warn('Unknown message type:', message.type);
        }
    }
    
    // Handle stock/crypto price data update
    function handlePriceData(data) {
        // Add a glitch animation for updated data
        const glitchAnimation = (element) => {
            element.classList.add('updated');
            setTimeout(() => {
                element.classList.remove('updated');
            }, 1000);
        };
        
        // Update UI for the specific symbol
        const allSymbols = elements.monitoredList.querySelectorAll('.monitored-symbol');
        for (const symbolElement of allSymbols) {
            const nameDiv = symbolElement.querySelector('.symbol-name');
            if (nameDiv && nameDiv.textContent === data.symbol) {
                const priceDiv = symbolElement.querySelector('.symbol-price');
                const changeDiv = symbolElement.querySelector('.symbol-change');
                
                // Add typing effect for price updates
                typeWriter(priceDiv, `$${data.price.toFixed(2)}`, 50);
                
                // Add change with direction indicator
                const changeText = `${data.changePercent >= 0 ? '↑' : '↓'} ${Math.abs(data.changePercent).toFixed(2)}%`;
                typeWriter(changeDiv, changeText, 50);
                
                changeDiv.classList.remove('positive', 'negative');
                changeDiv.classList.add(data.changePercent >= 0 ? 'positive' : 'negative');
                
                // Apply glitch animation to show it was updated
                glitchAnimation(symbolElement);
                break;
            }
        }
    }
    
    // Handle market index data update
    function handleIndexData(data) {
        // Update the UI for market indices
        elements.marketIndices.innerHTML = '';
        
        // Log received data for debugging
        console.log("Received market index data:", data);
        
        // Extract indices array from data
        const indices = data.indices || [];
        
        if (indices.length === 0) {
            console.error("No indices data found in the response");
            const placeholder = document.createElement('div');
            placeholder.className = 'market-index';
            placeholder.innerHTML = '<div class="index-name">Error</div><div class="index-value">No data available</div>';
            elements.marketIndices.appendChild(placeholder);
            return;
        }
        
        indices.forEach(index => {
            const clone = templates.marketIndex.content.cloneNode(true);
            const nameDiv = clone.querySelector('.index-name');
            const valueDiv = clone.querySelector('.index-value');
            const changeDiv = clone.querySelector('.index-change');
            
            nameDiv.textContent = index.name;
            valueDiv.textContent = index.value.toFixed(2);
            
            const changePercent = index.percent || (index.change / index.value * 100).toFixed(2);
            const changeText = `${index.change >= 0 ? '↑' : '↓'} ${Math.abs(index.change).toFixed(2)} (${Math.abs(changePercent).toFixed(2)}%)`;
            changeDiv.textContent = changeText;
            changeDiv.classList.add(index.change >= 0 ? 'positive' : 'negative');
            
            elements.marketIndices.appendChild(clone);
        });
        
        // Apply glitch animation to the indices
        elements.marketIndices.querySelectorAll('.market-index').forEach(element => {
            element.classList.add('updated');
            setTimeout(() => {
                element.classList.remove('updated');
            }, 1000);
        });
        
        // Update the state
        state.marketIndices = indices;
    }

    // Set up event listeners
    function setupEventListeners() {
        // Chat form submission
        elements.chatForm.addEventListener('submit', (e) => {
            e.preventDefault();
            const message = elements.chatInput.value.trim();
            if (message) {
                sendUserMessage(message);
                elements.chatInput.value = '';
                autoResizeTextarea(elements.chatInput);
            }
        });

        // Clear chat button
        elements.clearChat.addEventListener('click', () => {
            clearChat();
        });

        // Toggle theme button
        elements.toggleTheme.addEventListener('click', () => {
            toggleTheme();
        });

        // Add symbol button
        elements.addSymbolBtn.addEventListener('click', () => {
            const symbol = elements.symbolInput.value.trim().toUpperCase();
            if (symbol) {
                addMonitoredSymbol(symbol);
                elements.symbolInput.value = '';
            }
        });

        // Symbol input enter key
        elements.symbolInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                elements.addSymbolBtn.click();
            }
        });

        // Quick action buttons
        elements.quickActionBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                const query = btn.dataset.query;
                sendUserMessage(query);
                
                // Add a slight ripple effect on click
                const ripple = document.createElement('span');
                ripple.className = 'btn-ripple';
                btn.appendChild(ripple);
                setTimeout(() => ripple.remove(), 600);
            });
        });

        // Collapse panel button
        elements.collapsePanel.addEventListener('click', () => {
            togglePanel();
        });

        // Auto-resize textarea as user types
        elements.chatInput.addEventListener('input', () => {
            autoResizeTextarea(elements.chatInput);
        });
        
        // Add keyboard shortcuts
        document.addEventListener('keydown', (e) => {
            // Ctrl+K to clear chat
            if (e.ctrlKey && e.key === 'k') {
                e.preventDefault();
                clearChat();
            }
            
            // Ctrl+L to toggle theme
            if (e.ctrlKey && e.key === 'l') {
                e.preventDefault();
                toggleTheme();
            }
        });
    }

    // Send user message
    function sendUserMessage(content) {
        // Add message to UI
        addUserMessage(content);

        // Send to server
        if (state.websocket && state.websocket.readyState === WebSocket.OPEN) {
            state.websocket.send(JSON.stringify({
                type: 'user',
                content: content
            }));
        } else {
            addErrorMessage('> ERROR: Cannot send message. Connection lost.');
            setupWebSocket(); // Try to reconnect
        }

        // Check if it's a monitor command and handle locally
        if (content.toLowerCase().match(/^(monitor|track|alert)\s+([a-z0-9\-\s,]+)(\s+at\s+(\d+(\.\d+)?%)?)?/i)) {
            handleMonitorCommand(content);
        }
    }

    // Handle monitor commands
    function handleMonitorCommand(content) {
        // Extract symbols from the command
        const regex = /\b[A-Z]{1,5}(?:-[A-Z]{3})?\b/g;
        const matches = content.match(regex) || [];
        
        if (matches.length > 0) {
            // Extract threshold if specified
            let threshold = 1.0; // Default
            const thresholdRegex = /(\d+(\.\d+)?)%/;
            const thresholdMatches = content.match(thresholdRegex);
            if (thresholdMatches && thresholdMatches[1]) {
                threshold = parseFloat(thresholdMatches[1]);
            }
            
            // Send command to server with structured data
            if (state.websocket && state.websocket.readyState === WebSocket.OPEN) {
                state.websocket.send(JSON.stringify({
                    type: 'command',
                    content: 'monitor',
                    data: {
                        symbols: matches,
                        threshold: threshold
                    }
                }));
            }
            
            // Add symbols to monitored list
            matches.forEach(symbol => {
                addMonitoredSymbol(symbol);
            });
        }
    }

    // Add a user message to the chat
    function addUserMessage(content) {
        if (!elements.chatMessages) {
            console.error('Chat messages container not found');
            return;
        }
        
        const messageElement = createMessageElement('user', content);
        elements.chatMessages.appendChild(messageElement);
        scrollToBottom();
        
        // Add prefix to make it look command-like
        const contentDiv = messageElement.querySelector('.message-content');
        if (contentDiv) {
            contentDiv.innerHTML = `<span class="cmd-prompt">$</span> ${content}`;
        }
        
        // Save to history
        state.messageHistory.push({
            type: 'user',
            content: content,
            timestamp: new Date()
        });
    }

    // Add an assistant message to the chat with typing effect
    function addAssistantMessage(content) {
        if (!elements.chatMessages) {
            console.error('Chat messages container not found');
            return;
        }
        
        // Remove typing indicator if present
        removeTypingIndicator();
        
        // Create a very simple message element directly instead of using templates
        const messageElement = document.createElement('div');
        messageElement.className = 'message assistant';
        
        const contentDiv = document.createElement('div');
        contentDiv.className = 'message-content';
        
        const timeDiv = document.createElement('div');
        timeDiv.className = 'message-time';
        timeDiv.textContent = new Date().toLocaleTimeString();
        
        // Add terminal-like prefix
        if (state.isTypingEffect) {
            // Format content with Markdown
            const formattedContent = marked.parse(content);
            contentDiv.innerHTML = `<span class="cmd-prompt">></span> `;
            
            // Append the message elements first
            messageElement.appendChild(contentDiv);
            messageElement.appendChild(timeDiv);
            elements.chatMessages.appendChild(messageElement);
            
            // Use typewriter effect for the assistant messages
            typewriterHTML(contentDiv, formattedContent, 5);
        } else {
            contentDiv.innerHTML = `<span class="cmd-prompt">></span> ${marked.parse(content)}`;
            
            // Append the message elements
            messageElement.appendChild(contentDiv);
            messageElement.appendChild(timeDiv);
            elements.chatMessages.appendChild(messageElement);
        }
        
        scrollToBottom();
        
        // Save to history
        state.messageHistory.push({
            type: 'assistant',
            content: content,
            timestamp: new Date()
        });
    }

    // Add a system message to the chat
    function addSystemMessage(content) {
        // Check if chat messages container exists
        if (!elements.chatMessages) {
            console.error('Chat messages container not found');
            return;
        }
        
        const messageElement = createMessageElement('system', content);
        elements.chatMessages.appendChild(messageElement);
        
        // Add terminal prefix to make it look like a system message
        const contentDiv = messageElement.querySelector('.message-content');
        if (contentDiv) {
            contentDiv.innerHTML = `<span class="cmd-prompt">#</span> ${content}`;
        }
        
        scrollToBottom();
        
        // Save to history
        state.messageHistory.push({
            type: 'system',
            content: content,
            timestamp: new Date()
        });
    }
    
    // Add a tool usage message (collapsible and muted)
    function addToolMessage(content) {
        // Check if chat messages container exists
        if (!elements.chatMessages) {
            console.error('Chat messages container not found');
            return;
        }
        
        const messageElement = createMessageElement('tool', content);
        elements.chatMessages.appendChild(messageElement);
        
        // Format as a collapsible tool message
        const contentDiv = messageElement.querySelector('.message-content');
        if (contentDiv) {
            // Create a collapsible section
            const detailsElement = document.createElement('details');
            const summaryElement = document.createElement('summary');
            
            // Add tool usage icon and summary
            summaryElement.innerHTML = `<span class="cmd-prompt">$</span> <span class="tool-summary">Tool Usage (click to expand)</span>`;
            
            // Add the actual content in a pre tag
            const preElement = document.createElement('pre');
            preElement.className = 'tool-content';
            preElement.textContent = content;
            
            // Assemble the elements
            detailsElement.appendChild(summaryElement);
            detailsElement.appendChild(preElement);
            contentDiv.appendChild(detailsElement);
        }
        
        scrollToBottom();
    }

    // Add an error message to the chat
    function addErrorMessage(content) {
        // Check if chat messages container exists
        if (!elements.chatMessages) {
            console.error('Chat messages container not found');
            return;
        }
        
        // Remove typing indicator if present
        removeTypingIndicator();
        
        const messageElement = createMessageElement('error', content);
        elements.chatMessages.appendChild(messageElement);
        
        // Add error prefix
        const contentDiv = messageElement.querySelector('.message-content');
        if (contentDiv) {
            contentDiv.innerHTML = `<span class="cmd-prompt">!</span> ${content}`;
        }
        
        scrollToBottom();
        
        // Save to history
        state.messageHistory.push({
            type: 'error',
            content: content,
            timestamp: new Date()
        });
    }

    // Create a message element
    function createMessageElement(type, content) {
        // Check if template exists
        if (!templates.message) {
            console.error('Message template not found');
            // Create a simple message element as fallback
            const div = document.createElement('div');
            div.className = `message ${type}`;
            div.innerHTML = `<div class="message-content">${content}</div><div class="message-time">${new Date().toLocaleTimeString()}</div>`;
            return div;
        }
        
        try {
            const clone = templates.message.content.cloneNode(true);
            const messageDiv = clone.querySelector('.message');
            const contentDiv = clone.querySelector('.message-content');
            const timeDiv = clone.querySelector('.message-time');
            
            if (messageDiv) messageDiv.classList.add(type);
            
            // Early return with fallback if contentDiv is not found
            if (!contentDiv) {
                console.error('Message content div not found in template clone');
                // Create a fallback div with the content
                const div = document.createElement('div');
                div.className = `message ${type}`;
                div.innerHTML = `<div class="message-content">${content}</div><div class="message-time">${new Date().toLocaleTimeString()}</div>`;
                return div;
            }
            
            // Add timestamp
            if (timeDiv) {
                const now = new Date();
                timeDiv.textContent = now.toLocaleTimeString();
            }
            
            // For user messages, error messages, and system messages add content directly
            if (type === 'user' || type === 'error' || type === 'system') {
                contentDiv.textContent = content;
            }
            // For assistant messages, we don't set content here - it will be set in addAssistantMessage
            
            return clone;
        } catch (err) {
            console.error('Error creating message element:', err);
            // Create a simple message element as fallback
            const div = document.createElement('div');
            div.className = `message ${type}`;
            div.innerHTML = `<div class="message-content">${content}</div><div class="message-time">${new Date().toLocaleTimeString()}</div>`;
            return div;
        }
    }

    // Typewriter effect for normal text
    function typeWriter(element, text, speed = 30) {
        let i = 0;
        element.textContent = ''; // Clear first
        
        const typing = setInterval(() => {
            if (i < text.length) {
                element.textContent += text.charAt(i);
                i++;
            } else {
                clearInterval(typing);
            }
        }, speed);
    }
    
    // Typewriter effect that preserves HTML formatting
    function typewriterHTML(element, html, speed = 10) {
        // Create a temporary div to hold the HTML
        const tempDiv = document.createElement('div');
        tempDiv.innerHTML = html;
        
        // Convert the HTML into a flat array of text and HTML nodes
        const nodes = flattenNodes(tempDiv);
        let currentIndex = 0;
        
        // Start the typing effect
        const typing = setInterval(() => {
            if (currentIndex < nodes.length) {
                const node = nodes[currentIndex];
                
                if (node.type === 'text') {
                    element.innerHTML += node.content;
                } else if (node.type === 'tag') {
                    element.innerHTML += node.content;
                }
                
                currentIndex++;
                scrollToBottom();
            } else {
                clearInterval(typing);
            }
        }, speed);
    }
    
    // Flatten HTML nodes for the typewriter effect
    function flattenNodes(parentNode) {
        let result = [];
        
        // Process each child node
        parentNode.childNodes.forEach(node => {
            if (node.nodeType === Node.TEXT_NODE) {
                // Split text into characters
                const chars = node.textContent.split('');
                chars.forEach(char => {
                    result.push({ type: 'text', content: char });
                });
            } else if (node.nodeType === Node.ELEMENT_NODE) {
                // Add opening tag
                const openTag = getOpeningTag(node);
                result.push({ type: 'tag', content: openTag });
                
                // Process children recursively
                const childNodes = flattenNodes(node);
                result = result.concat(childNodes);
                
                // Add closing tag
                const closeTag = `</${node.tagName.toLowerCase()}>`;
                result.push({ type: 'tag', content: closeTag });
            }
        });
        
        return result;
    }
    
    // Get opening tag with attributes
    function getOpeningTag(node) {
        const tagName = node.tagName.toLowerCase();
        let attributes = '';
        
        for (let i = 0; i < node.attributes.length; i++) {
            const attr = node.attributes[i];
            attributes += ` ${attr.name}="${attr.value}"`;
        }
        
        return `<${tagName}${attributes}>`;
    }

    // Handle typing indicator
    function handleTypingIndicator(status) {
        // Remove existing indicator if present
        removeTypingIndicator();
        
        if (status === 'start') {
            // Create typing indicator with terminal styling
            const indicator = document.createElement('div');
            indicator.className = 'message assistant typing-indicator';
            indicator.innerHTML = `
                        <div class="dot"></div>
                    </div>
                </div>
            `;
            
            elements.chatMessages.appendChild(indicator);
            scrollToBottom();
        }
    }

    // Remove typing indicator
    function removeTypingIndicator() {
        const indicator = elements.chatMessages.querySelector('.typing-indicator');
        if (indicator) {
            indicator.remove();
        }
    }

    // Clear chat messages
    function clearChat() {
        elements.chatMessages.innerHTML = '';
        state.messageHistory = [];
        addSystemMessage('Terminal cleared. Session reset.');
    }

    // Toggle theme
    function toggleTheme() {
        state.theme = state.theme === 'light' ? 'dark' : 'light';
        localStorage.setItem('theme', state.theme);
        applyTheme();
    }

    // Apply current theme
    function applyTheme() {
        if (state.theme === 'dark') {
            document.body.classList.remove('light-theme');
            elements.toggleTheme.textContent = '☀️';
            // Update Chart.js theme if we have a chart
            updateChartTheme();
        } else {
            document.body.classList.add('light-theme');
            elements.toggleTheme.textContent = '🌙';
            // Update Chart.js theme if we have a chart
            updateChartTheme();
        }
    }
    
    // Update Chart.js theme based on current theme
    function updateChartTheme() {
        if (state.currentChart) {
            const textColor = state.theme === 'dark' ? '#f8fafc' : '#0f172a';
            const gridColor = state.theme === 'dark' ? '#334155' : '#e2e8f0';
            
            state.currentChart.options.plugins.tooltip.backgroundColor = state.theme === 'dark' ? '#1e293b' : '#ffffff';
            state.currentChart.options.plugins.tooltip.titleColor = textColor;
            state.currentChart.options.plugins.tooltip.bodyColor = textColor;
            state.currentChart.options.scales.x.grid.color = gridColor;
            state.currentChart.options.scales.x.ticks.color = textColor;
            state.currentChart.options.scales.y.grid.color = gridColor;
            state.currentChart.options.scales.y.ticks.color = textColor;
            
            state.currentChart.update();
        }
    }

    // Toggle data panel
    function togglePanel() {
        elements.dataPanel.classList.toggle('collapsed');
        state.isPanelCollapsed = elements.dataPanel.classList.contains('collapsed');
        localStorage.setItem('isPanelCollapsed', state.isPanelCollapsed);
    }

    // Add a symbol to monitored list
    function addMonitoredSymbol(symbol) {
        // Check if already monitoring
        if (state.monitoredSymbols.includes(symbol)) {
            return;
        }
        
        // Add to state
        state.monitoredSymbols.push(symbol);
        localStorage.setItem('monitoredSymbols', JSON.stringify(state.monitoredSymbols));
        
        // Update UI
        updateMonitoredSymbols();
    }

    // Remove a symbol from monitored list
    function removeMonitoredSymbol(symbol) {
        // Remove from state
        state.monitoredSymbols = state.monitoredSymbols.filter(s => s !== symbol);
        localStorage.setItem('monitoredSymbols', JSON.stringify(state.monitoredSymbols));
        
        // Update UI
        updateMonitoredSymbols();
    }

    // Update monitored symbols list
    function updateMonitoredSymbols() {
        if (!elements.monitoredList) {
            console.error('Monitored list element not found');
            return;
        }
        
        // Clear the list once - no duplicates
        elements.monitoredList.innerHTML = '';
        
        if (state.monitoredSymbols.length === 0) {
            const emptyState = document.createElement('p');
            emptyState.className = 'empty-state';
            emptyState.innerHTML = 'No symbols monitored yet';
            elements.monitoredList.appendChild(emptyState);
            return;
        }
        
        // Create a simple method of monitoring symbols without using templates
        state.monitoredSymbols.forEach(symbol => {
            // Create a direct DOM element instead of using templates
            const symbolDiv = document.createElement('div');
            symbolDiv.className = 'monitored-symbol';
            
            const nameDiv = document.createElement('div');
            nameDiv.className = 'symbol-name';
            nameDiv.textContent = symbol;
            
            const priceDiv = document.createElement('div');
            priceDiv.className = 'symbol-price';
            priceDiv.textContent = 'Loading...';
            
            const changeDiv = document.createElement('div');
            changeDiv.className = 'symbol-change';
            
            const removeBtn = document.createElement('button');
            removeBtn.className = 'remove-symbol';
            removeBtn.innerHTML = '<i data-lucide="x" size="12"></i>';
            removeBtn.addEventListener('click', (e) => {
                e.preventDefault();
                e.stopPropagation();
                removeMonitoredSymbol(symbol);
            });
            
            // Append all elements to the symbol div
            symbolDiv.appendChild(nameDiv);
            symbolDiv.appendChild(priceDiv);
            symbolDiv.appendChild(changeDiv);
            symbolDiv.appendChild(removeBtn);
            
            // Add the complete symbol div to the monitored list
            elements.monitoredList.appendChild(symbolDiv);
            
            // Initialize Lucide icons if available
            if (typeof lucide !== 'undefined' && lucide.createIcons) {
                lucide.createIcons({
                    attrs: {
                        'stroke-width': 1.5,
                        'stroke': 'currentColor'
                    },
                    elements: symbolDiv.querySelectorAll('[data-lucide]')
                });
            }
            
            // Fetch latest price data
            fetchSymbolPrice(symbol);
        });
    }

    // Fetch symbol price from API via WebSocket
    function fetchSymbolPrice(symbol) {
        // Request the price data via WebSocket command
        if (state.websocket && state.websocket.readyState === WebSocket.OPEN) {
            state.websocket.send(JSON.stringify({
                type: 'command',
                content: 'fetch_price',
                data: {
                    symbol: symbol
                }
            }));
        }
        
        // Show loading indicator until data arrives
        const allSymbols = elements.monitoredList.querySelectorAll('.monitored-symbol');
        for (const symbolElement of allSymbols) {
            const nameDiv = symbolElement.querySelector('.symbol-name');
            if (nameDiv && nameDiv.textContent === symbol) {
                const priceDiv = symbolElement.querySelector('.symbol-price');
                const changeDiv = symbolElement.querySelector('.symbol-change');
                
                priceDiv.textContent = 'Loading...';
                changeDiv.textContent = '';
                break;
            }
        }
    }

    // Fetch market indices from API via WebSocket
    function fetchMarketIndices() {
        // Request the market indices data via WebSocket command
        if (state.websocket && state.websocket.readyState === WebSocket.OPEN) {
            state.websocket.send(JSON.stringify({
                type: 'command',
                content: 'fetch_indices',
                data: {
                    indices: ['S&P 500', 'DOW', 'NASDAQ']
                }
            }));
        }
        
        // Show loading indicators until data arrives
        elements.marketIndices.innerHTML = '';
        ['S&P 500', 'DOW', 'NASDAQ'].forEach(indexName => {
            const clone = templates.marketIndex.content.cloneNode(true);
            const nameDiv = clone.querySelector('.index-name');
            const valueDiv = clone.querySelector('.index-value');
            const changeDiv = clone.querySelector('.index-change');
            
            nameDiv.textContent = indexName;
            valueDiv.textContent = 'Loading...';
            changeDiv.textContent = '';
            
            elements.marketIndices.appendChild(clone);
        });
    }

    // Handle alert message
    function handleAlert(alert) {
        console.log('Alert received:', alert);
        
        // Create alert element with cyberpunk-terminal styling
        const alertElement = templates.alert.content.cloneNode(true).firstElementChild;
        const symbolDiv = alertElement.querySelector('.alert-symbol');
        const priceDiv = alertElement.querySelector('.alert-price');
        const changeDiv = alertElement.querySelector('.alert-change');
        const closeBtn = alertElement.querySelector('.alert-close');
        
        symbolDiv.innerHTML = `<span class="cmd-prompt">$</span> ${alert.symbol}`;
        priceDiv.textContent = `$${alert.price.toFixed(2)}`;
        
        const directionArrow = alert.percentChange >= 0 ? '↑' : '↓';
        const changeText = `${directionArrow} ${Math.abs(alert.percentChange).toFixed(2)}% from $${alert.previousPrice.toFixed(2)}`;
        changeDiv.textContent = changeText;
        changeDiv.classList.add(alert.percentChange >= 0 ? 'positive' : 'negative');
        
        // Set up close button
        closeBtn.addEventListener('click', () => {
            document.body.removeChild(alertElement);
        });
        
        // Add to DOM
        document.body.appendChild(alertElement);
        alertElement.style.display = 'block';
        
        // Add glitch animation on appearance
        setTimeout(() => {
            alertElement.classList.add('glitch');
            setTimeout(() => alertElement.classList.remove('glitch'), 500);
        }, 100);
        
        // Auto-remove after 10 seconds
        setTimeout(() => {
            if (document.body.contains(alertElement)) {
                document.body.removeChild(alertElement);
            }
        }, 10000);
        
        // Also add as a message in the chat with terminal styling
        const alertContent = `**📊 PRICE ALERT**\n\n**${alert.symbol}** has ${alert.direction} by **${Math.abs(alert.percentChange).toFixed(2)}%**\nPrice: $${alert.previousPrice.toFixed(2)} → $${alert.price.toFixed(2)}\nVolume: ${alert.volume.toLocaleString()}`;
        
        const messageElement = createMessageElement('alert', alertContent);
        const contentDiv = messageElement.querySelector('.message-content');
        contentDiv.innerHTML = marked.parse(alertContent);
        
        elements.chatMessages.appendChild(messageElement);
        scrollToBottom();
    }

    // Visualize financial data from message content
    function visualizeFinancialData(content) {
        // Check for stock data tables
        const stockTableRegex = /## Stock Data.*?\| Symbol \| Price \| Change \| Change % \| Volume \| Timestamp \|[\s\S]*?\n\n/i;
        const stockTableMatch = content.match(stockTableRegex);
        
        if (stockTableMatch) {
            const tableContent = stockTableMatch[0];
            
            // Extract rows
            const rowRegex = /\| ([A-Z]+) \| \$([0-9.]+) \| (-?[0-9.]+) \| (-?[0-9.]+)% \| ([0-9,]+) \| ([0-9-T:Z.]+) \|/g;
            const stocks = [];
            let match;
            
            while ((match = rowRegex.exec(tableContent)) !== null) {
                stocks.push({
                    symbol: match[1],
                    price: parseFloat(match[2]),
                    change: parseFloat(match[3]),
                    changePercent: parseFloat(match[4]),
                    volume: match[5],
                    timestamp: match[6]
                });
            }
            
            if (stocks.length > 0) {
                // Create price chart
                createPriceChart(stocks);
                
                // Create data cards
                updateDataCards(stocks);
                
                // Expand data panel if collapsed
                if (state.isPanelCollapsed) {
                    togglePanel();
                }
            }
        }
        
        // Check for crypto data tables (similar processing)
        const cryptoTableRegex = /## Cryptocurrency Data.*?\| Symbol \| Price \| Change \| Change % \| Volume \| Timestamp \|[\s\S]*?\n\n/i;
        const cryptoTableMatch = content.match(cryptoTableRegex);
        
        if (cryptoTableMatch) {
            const tableContent = cryptoTableMatch[0];
            
            // Extract rows
            const rowRegex = /\| ([A-Z-]+) \| \$([0-9.]+) \| (-?[0-9.]+) \| (-?[0-9.]+)% \| ([0-9,]+) \| ([0-9-T:Z.]+) \|/g;
            const cryptos = [];
            let match;
            
            while ((match = rowRegex.exec(tableContent)) !== null) {
                cryptos.push({
                    symbol: match[1],
                    price: parseFloat(match[2]),
                    change: parseFloat(match[3]),
                    changePercent: parseFloat(match[4]),
                    volume: match[5],
                    timestamp: match[6]
                });
            }
            
            if (cryptos.length > 0) {
                // Create price chart
                createPriceChart(cryptos);
                
                // Create data cards
                updateDataCards(cryptos);
                
                // Expand data panel if collapsed
                if (state.isPanelCollapsed) {
                    togglePanel();
                }
            }
        }
    }

    // Create price chart for financial data
    function createPriceChart(data) {
        // Destroy existing chart if any
        if (state.currentChart) {
            state.currentChart.destroy();
        }
        
        // Process data for chart
        const labels = data.map(item => item.symbol);
        const prices = data.map(item => item.price);
        const changes = data.map(item => item.changePercent);
        
        // Use theme-based colors
        const isDark = state.theme === 'dark';
        const positiveColor = isDark ? 'rgba(16, 185, 129, 0.7)' : 'rgba(16, 185, 129, 0.7)';
        const negativeColor = isDark ? 'rgba(239, 68, 68, 0.7)' : 'rgba(239, 68, 68, 0.7)';
        const textColor = isDark ? '#f8fafc' : '#0f172a';
        const gridColor = isDark ? '#334155' : '#e2e8f0';
        
        const backgroundColors = changes.map(change => 
            change >= 0 ? positiveColor : negativeColor
        );
        
        // Chart.js configuration with terminal-like styling
        const ctx = elements.priceChart.getContext('2d');
        state.currentChart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Price ($)',
                    data: prices,
                    backgroundColor: backgroundColors,
                    borderColor: backgroundColors.map(color => color.replace('0.7', '1')),
                    borderWidth: 1,
                    borderRadius: 4,
                    barThickness: 18,
                    maxBarThickness: 30
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: {
                    duration: 1000,
                    easing: 'easeOutQuart'
                },
                plugins: {
                    legend: {
                        display: false
                    },
                    tooltip: {
                        backgroundColor: isDark ? '#1e293b' : '#ffffff',
                        titleColor: textColor,
                        bodyColor: textColor,
                        cornerRadius: 6,
                        padding: 12,
                        displayColors: false,
                        titleFont: {
                            family: "'JetBrains Mono', monospace",
                            size: 14
                        },
                        bodyFont: {
                            family: "'JetBrains Mono', monospace",
                            size: 12
                        },
                        callbacks: {
                            title: function(tooltipItems) {
                                return tooltipItems[0].label;
                            },
                            label: function(context) {
                                const index = context.dataIndex;
                                const arrow = changes[index] >= 0 ? '↑' : '↓';
                                return [
                                    `Price: $${prices[index].toFixed(2)}`,
                                    `Change: ${arrow} ${Math.abs(changes[index]).toFixed(2)}%`
                                ];
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        grid: {
                            display: false,
                            color: gridColor
                        },
                        ticks: {
                            color: textColor,
                            font: {
                                family: "'JetBrains Mono', monospace",
                                size: 11
                            }
                        }
                    },
                    y: {
                        beginAtZero: false,
                        grid: {
                            color: gridColor,
                            lineWidth: 0.5
                        },
                        ticks: {
                            color: textColor,
                            font: {
                                family: "'JetBrains Mono', monospace",
                                size: 11
                            },
                            callback: function(value) {
                                return '$' + value;
                            }
                        }
                    }
                }
            }
        });
    }

    // Update data cards with financial data
    function updateDataCards(data) {
        elements.dataCards.innerHTML = '';
        
        // Calculate average price and change
        const avgPrice = data.reduce((sum, item) => sum + item.price, 0) / data.length;
        const avgChange = data.reduce((sum, item) => sum + item.changePercent, 0) / data.length;
        
        // Create average price card
        const avgPriceCard = templates.dataCard.content.cloneNode(true);
        const avgPriceHeader = avgPriceCard.querySelector('.card-header');
        const avgPriceValue = avgPriceCard.querySelector('.card-value');
        
        avgPriceHeader.textContent = 'AVG PRICE';
        avgPriceValue.textContent = `$${avgPrice.toFixed(2)}`;
        
        elements.dataCards.appendChild(avgPriceCard);
        
        // Create average change card
        const avgChangeCard = templates.dataCard.content.cloneNode(true);
        const avgChangeHeader = avgChangeCard.querySelector('.card-header');
        const avgChangeValue = avgChangeCard.querySelector('.card-value');
        const avgChangeEl = avgChangeCard.querySelector('.card-change');
        
        avgChangeHeader.textContent = 'AVG CHANGE';
        
        const directionArrow = avgChange >= 0 ? '↑' : '↓';
        avgChangeValue.textContent = `${directionArrow} ${Math.abs(avgChange).toFixed(2)}%`;
        avgChangeEl.classList.add(avgChange >= 0 ? 'positive' : 'negative');
        
        elements.dataCards.appendChild(avgChangeCard);
        
        // Create highest gainer card
        const sortedByChange = [...data].sort((a, b) => b.changePercent - a.changePercent);
        const highestGainer = sortedByChange[0];
        
        const gainerCard = templates.dataCard.content.cloneNode(true);
        const gainerHeader = gainerCard.querySelector('.card-header');
        const gainerValue = gainerCard.querySelector('.card-value');
        const gainerChange = gainerCard.querySelector('.card-change');
        
        gainerHeader.textContent = 'TOP PERFORMER';
        gainerValue.textContent = highestGainer.symbol;
        
        const gainerArrow = highestGainer.changePercent >= 0 ? '↑' : '↓';
        gainerChange.textContent = `${gainerArrow} ${Math.abs(highestGainer.changePercent).toFixed(2)}%`;
        gainerChange.classList.add(highestGainer.changePercent >= 0 ? 'positive' : 'negative');
        
        elements.dataCards.appendChild(gainerCard);
        
        // Create market cap or volume card
        const volumeCard = templates.dataCard.content.cloneNode(true);
        const volumeHeader = volumeCard.querySelector('.card-header');
        const volumeValue = volumeCard.querySelector('.card-value');
        
        volumeHeader.textContent = 'TOTAL VOLUME';
        
        // Try to parse volume as number
        const totalVolume = data.reduce((sum, item) => {
            const volume = typeof item.volume === 'string' 
                ? parseInt(item.volume.replace(/,/g, ''))
                : item.volume;
            return sum + (isNaN(volume) ? 0 : volume);
        }, 0);
        
        volumeValue.textContent = totalVolume.toLocaleString();
        
        elements.dataCards.appendChild(volumeCard);
        
        // Add a glitch animation effect to the cards
        setTimeout(() => {
            const cards = elements.dataCards.querySelectorAll('.data-card');
            cards.forEach((card, index) => {
                setTimeout(() => {
                    card.classList.add('updated');
                    setTimeout(() => card.classList.remove('updated'), 1000);
                }, index * 200);
            });
        }, 200);
    }

    // Update data panel
    function updateDataPanel() {
        if (!state.isPanelCollapsed) {
            // Show some placeholder content
            elements.dataCards.innerHTML = '';
            
            const placeholderCard = templates.dataCard.content.cloneNode(true);
            const placeholderHeader = placeholderCard.querySelector('.card-header');
            const placeholderValue = placeholderCard.querySelector('.card-value');
            
            placeholderHeader.textContent = 'NO DATA YET';
            placeholderValue.innerHTML = 'Ask about markets';
            
            elements.dataCards.appendChild(placeholderCard);
        }
    }

    // Auto-resize textarea
    function autoResizeTextarea(textarea) {
        textarea.style.height = 'auto';
        textarea.style.height = (textarea.scrollHeight) + 'px';
    }

    // Scroll chat to bottom
    function scrollToBottom() {
        elements.chatMessages.scrollTop = elements.chatMessages.scrollHeight;
    }

    // Initialize app
    init();
});