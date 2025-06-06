// Training page JavaScript
console.log('🧠 Training page loaded!');

// Page-specific variables
let trainingInProgress = false;
let trainingProgress = 0;
let trainingLogs = [];

// Initialize training page
document.addEventListener('DOMContentLoaded', function() {
    console.log('🎯 Initializing training page...');
    
    // Load initial data
    loadTrainingStatus();
    loadTrainingHistory();
    loadTrainingLogs();
    
    // Setup event listeners
    setupTrainingEventListeners();
    
    // Check training status every 30 seconds
    setInterval(() => {
        if (!isLoading) {
            loadTrainingStatus();
            if (trainingInProgress) {
                loadTrainingLogs();
            }
        }
    }, 30000);
    
    console.log('✅ Training page initialized!');
});

// Setup event listeners
function setupTrainingEventListeners() {
    // Log level filter
    const logLevel = document.getElementById('logLevel');
    if (logLevel) {
        logLevel.addEventListener('change', filterLogs);
    }
}

// Load training status
async function loadTrainingStatus() {
    try {
        console.log('🧠 Loading training status...');
        
        const response = await apiCall('/training/status');
        
        if (response.success && response.data) {
            updateTrainingStatus(response.data);
        } else {
            throw new Error('API not available');
        }
        
    } catch (error) {
        console.error('❌ API not available, using mock status:', error);
        updateMockTrainingStatus();
    }
}

// Update training status display
function updateTrainingStatus(data) {
    const statusIndicator = document.getElementById('statusIndicator');
    const statusText = document.getElementById('statusText');
    const lastTraining = document.getElementById('lastTraining');
    const nextTraining = document.getElementById('nextTraining');
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');
    const startBtn = document.getElementById('startTrainingBtn');
    const stopBtn = document.getElementById('stopTrainingBtn');
    
    // Update status
    trainingInProgress = data.is_training;
    trainingProgress = data.progress || 0;
    
    if (statusIndicator && statusText) {
        const statusDot = statusIndicator.querySelector('.status-dot');
        if (data.is_training) {
            statusDot.className = 'status-dot training';
            statusText.textContent = 'Training in Progress';
        } else {
            statusDot.className = 'status-dot idle';
            statusText.textContent = 'Idle';
        }
    }
    
    // Update times
    if (lastTraining) {
        lastTraining.textContent = formatRelativeTime(data.last_trained);
    }
    if (nextTraining) {
        nextTraining.textContent = formatDateTime(data.next_training);
    }
    
    // Update progress
    if (progressFill) {
        progressFill.style.width = `${trainingProgress}%`;
    }
    if (progressText) {
        progressText.textContent = `${trainingProgress.toFixed(1)}%`;
    }
    
    // Update buttons
    if (startBtn && stopBtn) {
        startBtn.disabled = data.is_training;
        stopBtn.disabled = !data.is_training;
    }
}

// Update mock training status
function updateMockTrainingStatus() {
    const mockData = {
        is_training: false,
        last_trained: '2024-01-14T02:00:00Z',
        next_training: '2024-01-21T09:00:00Z',
        progress: 0,
        current_phase: 'idle'
    };
    
    updateTrainingStatus(mockData);
}

// Load training history
async function loadTrainingHistory() {
    showLoading('historyLoading');
    
    try {
        console.log('📊 Loading training history...');
        
        const response = await apiCall('/training/history?limit=10');
        
        if (response.success && response.data) {
            renderTrainingHistory(response.data.sessions || []);
        } else {
            throw new Error('API not available');
        }
        
    } catch (error) {
        console.error('❌ API not available, using sample data:', error);
        renderSampleTrainingHistory();
    } finally {
        hideLoading('historyLoading');
    }
}

// Render training history
function renderTrainingHistory(sessions) {
    const tbody = document.getElementById('historyTableBody');
    if (!tbody) return;
    
    if (!sessions || sessions.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="no-data">📊 No training history available</td></tr>';
        return;
    }
    
    tbody.innerHTML = sessions.map(session => `
        <tr onclick="showTrainingDetails(${session.id})" style="cursor: pointer;">
            <td>${formatDate(session.start_time)}</td>
            <td>${formatDuration(session.duration_ms)}</td>
            <td>${session.total_stocks}</td>
            <td class="change positive">${session.success_count}</td>
            <td class="change ${session.error_count > 0 ? 'negative' : 'neutral'}">${session.error_count}</td>
            <td>${session.overall_accuracy ? session.overall_accuracy.toFixed(1) + '%' : '-'}</td>
            <td><span class="status ${session.status}">${session.status}</span></td>
            <td><button class="action-btn small" onclick="event.stopPropagation(); showSessionDetails(${session.id})">Details</button></td>
        </tr>
    `).join('');
}

// Render sample training history
function renderSampleTrainingHistory() {
    const sampleSessions = [
        { id: 1, start_time: '2024-01-14T09:00:00Z', duration_ms: 320000, total_stocks: 30, success_count: 28, error_count: 2, overall_accuracy: 87.3, status: 'completed' },
        { id: 2, start_time: '2024-01-07T09:00:00Z', duration_ms: 290000, total_stocks: 30, success_count: 30, error_count: 0, overall_accuracy: 89.1, status: 'completed' },
        { id: 3, start_time: '2023-12-31T09:00:00Z', duration_ms: 350000, total_stocks: 30, success_count: 25, error_count: 5, overall_accuracy: 82.4, status: 'completed' },
        { id: 4, start_time: '2023-12-24T09:00:00Z', duration_ms: 180000, total_stocks: 30, success_count: 15, error_count: 15, overall_accuracy: 45.2, status: 'partial' }
    ];
    
    renderTrainingHistory(sampleSessions);
}

// Load training logs
async function loadTrainingLogs() {
    try {
        console.log('📝 Loading training logs...');
        
        // Mock logs for now
        const sampleLogs = [
            { timestamp: '2024-01-15 14:30:25', level: 'INFO', message: '🚀 Starting weekly model training cronjob...' },
            { timestamp: '2024-01-15 14:30:26', level: 'INFO', message: '📊 Training models on 30 VN30 stocks' },
            { timestamp: '2024-01-15 14:32:15', level: 'SUCCESS', message: '✅ LSTM training completed: 28/30 successful, accuracy: 93.20%' },
            { timestamp: '2024-01-15 14:34:02', level: 'WARNING', message: '⚠️ ARIMA training had 7 errors for low-volume stocks' },
            { timestamp: '2024-01-15 14:35:45', level: 'SUCCESS', message: '🎉 Weekly training completed in 5m 20s' }
        ];
        
        trainingLogs = sampleLogs;
        renderTrainingLogs(trainingLogs);
        
    } catch (error) {
        console.error('❌ Error loading training logs:', error);
    }
}

// Render training logs
function renderTrainingLogs(logs) {
    const container = document.getElementById('trainingLogs');
    if (!container) return;
    
    container.innerHTML = logs.map(log => `
        <div class="log-entry ${log.level.toLowerCase()}">
            <span class="timestamp">${log.timestamp}</span>
            <span class="level">${log.level}</span>
            <span class="message">${log.message}</span>
        </div>
    `).join('');
    
    // Auto-scroll to bottom
    container.scrollTop = container.scrollHeight;
}

// Filter logs by level
function filterLogs() {
    const selectedLevel = document.getElementById('logLevel')?.value;
    
    let filteredLogs = trainingLogs;
    
    if (selectedLevel && selectedLevel !== 'all') {
        filteredLogs = trainingLogs.filter(log => 
            log.level.toLowerCase() === selectedLevel.toLowerCase()
        );
    }
    
    renderTrainingLogs(filteredLogs);
}

// Start training
async function startTraining() {
    if (trainingInProgress) {
        showNotification('Training is already in progress!', 'warning');
        return;
    }
    
    try {
        console.log('🚀 Starting training...');
        
        // In real app, this would call API
        showNotification('Training started! This may take several minutes...', 'info');
        
        // Mock training progress
        simulateTraining();
        
    } catch (error) {
        console.error('❌ Error starting training:', error);
        showNotification('Failed to start training!', 'error');
    }
}

// Stop training
async function stopTraining() {
    if (!trainingInProgress) {
        showNotification('No training in progress!', 'warning');
        return;
    }
    
    try {
        console.log('⏹️ Stopping training...');
        
        // In real app, this would call API
        showNotification('Training stopped!', 'info');
        
        trainingInProgress = false;
        updateTrainingButtons();
        
    } catch (error) {
        console.error('❌ Error stopping training:', error);
        showNotification('Failed to stop training!', 'error');
    }
}

// Train specific algorithm
async function trainAlgorithm(algorithmName) {
    console.log(`🤖 Training ${algorithmName} algorithm...`);
    
    try {
        // In real app, this would call API
        showNotification(`Training ${algorithmName.toUpperCase()} algorithm...`, 'info');
        
        // Mock training process
        setTimeout(() => {
            showNotification(`${algorithmName.toUpperCase()} training completed!`, 'success');
        }, 3000);
        
    } catch (error) {
        console.error(`❌ Error training ${algorithmName}:`, error);
        showNotification(`Failed to train ${algorithmName}!`, 'error');
    }
}

// Simulate training progress
function simulateTraining() {
    trainingInProgress = true;
    trainingProgress = 0;
    
    const progressInterval = setInterval(() => {
        trainingProgress += Math.random() * 10;
        
        if (trainingProgress >= 100) {
            trainingProgress = 100;
            trainingInProgress = false;
            clearInterval(progressInterval);
            showNotification('Training completed successfully!', 'success');
            loadTrainingHistory(); // Refresh history
        }
        
        updateProgressDisplay();
    }, 1000);
    
    updateTrainingButtons();
}

// Update progress display
function updateProgressDisplay() {
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');
    
    if (progressFill) {
        progressFill.style.width = `${trainingProgress}%`;
    }
    if (progressText) {
        progressText.textContent = `${trainingProgress.toFixed(1)}%`;
    }
}

// Update training buttons
function updateTrainingButtons() {
    const startBtn = document.getElementById('startTrainingBtn');
    const stopBtn = document.getElementById('stopTrainingBtn');
    
    if (startBtn && stopBtn) {
        startBtn.disabled = trainingInProgress;
        stopBtn.disabled = !trainingInProgress;
    }
}

// Refresh training history
function refreshTrainingHistory() {
    showNotification('Refreshing training history...', 'info');
    loadTrainingHistory();
}

// Refresh logs
function refreshLogs() {
    showNotification('Refreshing training logs...', 'info');
    loadTrainingLogs();
}

// Clear logs
function clearLogs() {
    if (confirm('Are you sure you want to clear all logs?')) {
        trainingLogs = [];
        renderTrainingLogs([]);
        showNotification('Training logs cleared!', 'success');
    }
}

// Show training details (placeholder)
function showTrainingDetails(sessionId) {
    console.log('📊 Show training details for session:', sessionId);
    showNotification(`Training details for session ${sessionId} - Coming soon!`, 'info');
}

// Show session details (placeholder)
function showSessionDetails(sessionId) {
    console.log('📝 Show session details for:', sessionId);
    showNotification(`Session details for ${sessionId} - Coming soon!`, 'info');
}

// Utility functions
function formatDuration(milliseconds) {
    const seconds = Math.floor(milliseconds / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    
    if (hours > 0) {
        return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
    } else if (minutes > 0) {
        return `${minutes}m ${seconds % 60}s`;
    } else {
        return `${seconds}s`;
    }
}

function formatRelativeTime(dateStr) {
    if (!dateStr) return 'Never';
    
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now - date;
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
    const diffMinutes = Math.floor(diffMs / (1000 * 60));
    
    if (diffDays > 0) {
        return `${diffDays} day${diffDays === 1 ? '' : 's'} ago`;
    } else if (diffHours > 0) {
        return `${diffHours} hour${diffHours === 1 ? '' : 's'} ago`;
    } else if (diffMinutes > 0) {
        return `${diffMinutes} minute${diffMinutes === 1 ? '' : 's'} ago`;
    } else {
        return 'Just now';
    }
}

function formatDateTime(dateStr) {
    if (!dateStr) return 'Not scheduled';
    
    const date = new Date(dateStr);
    return date.toLocaleDateString('vi-VN', {
        weekday: 'long',
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
    });
}

// Export functions for global access
window.startTraining = startTraining;
window.stopTraining = stopTraining;
window.trainAlgorithm = trainAlgorithm;
window.refreshTrainingHistory = refreshTrainingHistory;
window.refreshLogs = refreshLogs;
window.clearLogs = clearLogs;
window.showTrainingDetails = showTrainingDetails;
window.showSessionDetails = showSessionDetails;

console.log('🎉 Training JavaScript ready!');