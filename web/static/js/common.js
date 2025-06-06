// Common JavaScript functions for VN Stock Dashboard
console.log('🚀 Common JS loaded!');

// Global variables
const API_BASE = '/api';
let isLoading = false;

// Initialize common functionality
document.addEventListener('DOMContentLoaded', function() {
    console.log('📊 Initializing common functions...');
    
    // Update market status
    updateMarketStatus();
    
    // Setup periodic updates
    setInterval(updateMarketStatus, 60000); // Every minute
    
    // Handle navigation highlighting
    highlightCurrentPage();
    
    console.log('✅ Common functions initialized!');
});

// Market status management
function updateMarketStatus() {
    const now = new Date();
    const hour = now.getHours();
    const minute = now.getMinutes();
    const day = now.getDay(); // 0 = Sunday, 6 = Saturday
    
    const statusElement = document.getElementById('marketStatus');
    const statusText = document.getElementById('marketStatusText');
    
    if (!statusElement || !statusText) {
        console.log('Market status elements not found');
        return;
    }
    
    // Check if it's weekend
    if (day === 0 || day === 6) {
        statusElement.className = 'market-status closed';
        statusText.textContent = 'Market Closed (Weekend)';
        return;
    }
    
    // Vietnam market hours: 9:00-11:30 (morning), 13:00-15:00 (afternoon)
    const isOpen = (hour >= 9 && hour < 11) || 
                  (hour === 11 && minute <= 30) || 
                  (hour >= 13 && hour < 15);
    
    if (isOpen) {
        statusElement.className = 'market-status open';
        statusText.textContent = 'Market Open';
    } else if (hour < 9) {
        statusElement.className = 'market-status closed';
        statusText.textContent = 'Pre-Market';
    } else if ((hour === 11 && minute > 30) || hour === 12) {
        statusElement.className = 'market-status closed';
        statusText.textContent = 'Lunch Break';
    } else {
        statusElement.className = 'market-status closed';
        statusText.textContent = 'Market Closed';
    }
}

// Navigation highlighting
function highlightCurrentPage() {
    const currentPath = window.location.pathname;
    const navLinks = document.querySelectorAll('.nav-link');
    
    navLinks.forEach(link => {
        link.classList.remove('active');
        
        const href = link.getAttribute('href');
        if (href === currentPath || 
            (currentPath === '/' && href === '/') ||
            (currentPath === '/dashboard' && href === '/')) {
            link.classList.add('active');
        }
    });
}

// Utility Functions

// Format price in Vietnamese currency
function formatPrice(price) {
    if (typeof price === 'string') {
        price = parseFloat(price);
    }
    
    return new Intl.NumberFormat('vi-VN', {
        style: 'currency',
        currency: 'VND',
        minimumFractionDigits: 0,
        maximumFractionDigits: 0
    }).format(price || 0);
}

// Format price change with percentage
function formatChange(change, percent = null) {
    if (typeof change === 'string') {
        change = parseFloat(change);
    }
    
    const changeStr = change > 0 ? `+${formatPrice(Math.abs(change))}` : `-${formatPrice(Math.abs(change))}`;
    const percentStr = percent ? ` (${percent > 0 ? '+' : ''}${percent.toFixed(2)}%)` : '';
    return changeStr + percentStr;
}

// Get CSS class for price change
function getChangeClass(change) {
    if (typeof change === 'string') {
        change = parseFloat(change);
    }
    
    if (change > 0) return 'positive';
    if (change < 0) return 'negative';
    return 'neutral';
}

// Format date for display
function formatDate(dateStr) {
    if (!dateStr) return 'Tomorrow';
    
    try {
        const date = new Date(dateStr);
        const today = new Date();
        const tomorrow = new Date(today);
        tomorrow.setDate(tomorrow.getDate() + 1);
        
        if (date.toDateString() === today.toDateString()) {
            return 'Today';
        } else if (date.toDateString() === tomorrow.toDateString()) {
            return 'Tomorrow';
        } else {
            return date.toLocaleDateString('vi-VN', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        }
    } catch (error) {
        return 'Tomorrow';
    }
}

// Show/hide loading states
function showLoading(elementId) {
    const element = document.getElementById(elementId);
    if (element) {
        element.classList.add('show');
        isLoading = true;
    }
}

function hideLoading(elementId) {
    const element = document.getElementById(elementId);
    if (element) {
        element.classList.remove('show');
        isLoading = false;
    }
}

// Show notifications
function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    notification.innerHTML = `
        <div style="display: flex; align-items: center; gap: 10px;">
            <span>${getNotificationIcon(type)}</span>
            <span>${message}</span>
            <button onclick="this.parentElement.parentElement.remove()" style="margin-left: auto; background: none; border: none; font-size: 18px; cursor: pointer;">×</button>
        </div>
    `;
    
    document.body.appendChild(notification);
    
    // Show notification
    setTimeout(() => notification.classList.add('show'), 100);
    
    // Auto-hide after 5 seconds
    setTimeout(() => {
        notification.classList.remove('show');
        setTimeout(() => notification.remove(), 300);
    }, 5000);
}

function getNotificationIcon(type) {
    switch(type) {
        case 'success': return '✅';
        case 'error': return '❌';
        case 'warning': return '⚠️';
        default: return 'ℹ️';
    }
}

// API call wrapper
async function apiCall(endpoint, options = {}) {
    const url = `${API_BASE}${endpoint}`;
    
    try {
        const response = await fetch(url, {
            headers: {
                'Accept': 'application/json',
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const data = await response.json();
        return data;
    } catch (error) {
        console.error(`API call failed: ${endpoint}`, error);
        throw error;
    }
}

// Connection status monitoring
function checkConnection() {
    return navigator.onLine;
}

// Handle online/offline events
window.addEventListener('online', function() {
    showNotification('Connection restored! 🌐', 'success');
    console.log('🌐 Back online');
});

window.addEventListener('offline', function() {
    showNotification('Connection lost! Using cached data 📡', 'warning');
    console.log('📡 Gone offline');
});

// Performance monitoring
function measurePerformance(label, fn) {
    const start = performance.now();
    const result = fn();
    const end = performance.now();
    console.log(`⚡ ${label}: ${(end - start).toFixed(2)}ms`);
    return result;
}

// Local storage helpers
function saveToStorage(key, data) {
    try {
        localStorage.setItem(key, JSON.stringify(data));
    } catch (error) {
        console.warn('LocalStorage not available:', error);
    }
}

function getFromStorage(key) {
    try {
        const data = localStorage.getItem(key);
        return data ? JSON.parse(data) : null;
    } catch (error) {
        console.warn('LocalStorage read failed:', error);
        return null;
    }
}

// Debug helpers
function debugLog(message, data = null) {
    if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
        console.log(`🐛 ${message}`, data);
    }
}

// Export functions for global access
window.formatPrice = formatPrice;
window.formatChange = formatChange;
window.getChangeClass = getChangeClass;
window.formatDate = formatDate;
window.showLoading = showLoading;
window.hideLoading = hideLoading;
window.showNotification = showNotification;
window.apiCall = apiCall;

console.log('🎉 Common JavaScript ready!');