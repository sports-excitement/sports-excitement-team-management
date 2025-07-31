// Time Tracker Dashboard JavaScript
// Only initializes dashboard features on dashboard pages to avoid errors on login page

let ws = null;
let dataTable = null;
let statusChart = null;
let progressChart = null;

// Initialize dashboard when DOM is loaded
$(document).ready(function() {
    // Only initialize dashboard components if we're on the dashboard page
    if (isDashboardPage()) {
        initializeDataTable();
        initializeCharts();
        initializeWebSocket();
        
        // Auto-refresh every 30 seconds if WebSocket is not connected
        setInterval(function() {
            if (!ws || ws.readyState !== WebSocket.OPEN) {
                refreshData();
            }
        }, 30000);
        
        // Periodic data refresh every 2 minutes to keep working hours updated
        setInterval(function() {
            refreshData();
        }, 120000);
    }
});

// Check if we're on a dashboard page
function isDashboardPage() {
    return window.location.pathname === '/dashboard' || 
           window.location.pathname.startsWith('/dashboard') || 
           typeof window.dashboardData !== 'undefined';
}

// Helper function to get top performing users for charts
function getTopPerformingUsers(users, minHours = 10, limit = 10) {
    return users
        .filter(u => u.weekly_hours > minHours)
        .sort((a, b) => b.weekly_hours - a.weekly_hours)
        .slice(0, limit);
}

// Initialize DataTable
function initializeDataTable() {
    if ($.fn.DataTable.isDataTable('#usersTable')) {
        $('#usersTable').DataTable().destroy();
    }
    
    dataTable = $('#usersTable').DataTable({
        responsive: true,
        pageLength: 25,
        order: [[2, 'desc']], // Sort by status (Working first)
        dom: 'Bfrtip',
        buttons: [
            {
                extend: 'excel',
                text: '<i class="fas fa-file-excel me-1"></i>Excel',
                className: 'btn btn-success btn-sm',
                filename: 'time_tracker_users_' + new Date().toISOString().split('T')[0],
                title: 'Time Tracker - User Report'
            },
            {
                extend: 'csv',
                text: '<i class="fas fa-file-csv me-1"></i>CSV',
                className: 'btn btn-info btn-sm',
                filename: 'time_tracker_users_' + new Date().toISOString().split('T')[0]
            },
            {
                extend: 'print',
                text: '<i class="fas fa-print me-1"></i>Print',
                className: 'btn btn-secondary btn-sm',
                title: 'Time Tracker - User Report'
            }
        ],
        columnDefs: [
            {
                targets: [3, 4, 5], // Weekly, Monthly, Total hours columns
                render: function(data, type, row) {
                    if (type === 'export') {
                        return data;
                    }
                    return data;
                }
            }
        ],
        language: {
            search: "_INPUT_",
            searchPlaceholder: "Search users...",
            lengthMenu: "Show _MENU_ entries",
            info: "Showing _START_ to _END_ of _TOTAL_ users",
            infoEmpty: "No users found",
            infoFiltered: "(filtered from _MAX_ total users)"
        }
    });
}

// Initialize Charts
function initializeCharts() {
    initializeStatusChart();
    initializeProgressChart();
}

// Initialize Status Distribution Chart
function initializeStatusChart() {
    const ctx = document.getElementById('statusChart');
    if (!ctx) return;
    
    const users = window.dashboardData?.users || [];
    const activeUsers = users.filter(u => u.is_currently_working).length;
    const inactiveUsers = users.length - activeUsers;
    
    statusChart = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: ['Working', 'Offline'],
            datasets: [{
                data: [activeUsers, inactiveUsers],
                backgroundColor: [
                    '#28a745',
                    '#6c757d'
                ],
                borderWidth: 2,
                borderColor: '#fff'
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'bottom',
                    labels: {
                        padding: 20,
                        usePointStyle: true
                    }
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            const total = context.dataset.data.reduce((a, b) => a + b, 0);
                            const percentage = ((context.raw / total) * 100).toFixed(1);
                            return `${context.label}: ${context.raw} (${percentage}%)`;
                        }
                    }
                }
            }
        }
    });
}

// Initialize Weekly Progress Chart
function initializeProgressChart() {
    const ctx = document.getElementById('progressChart');
    if (!ctx) return;
    
    const users = window.dashboardData?.users || [];
    const topUsers = getTopPerformingUsers(users, 10, 10); // Min 10 hours, max 10 users
    
    const userNames = topUsers.map(u => u.name.split(' ')[0]); // First name only
    const weeklyHours = topUsers.map(u => u.weekly_hours);
    const target = 20; // 20 hours per week target
    
    progressChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: userNames,
            datasets: [
                {
                    label: 'Weekly Hours',
                    data: weeklyHours,
                    backgroundColor: weeklyHours.map(hours => 
                        hours >= target ? '#28a745' : 
                        hours >= target * 0.5 ? '#ffc107' : '#dc3545'
                    ),
                    borderColor: '#fff',
                    borderWidth: 1
                },
                {
                    label: 'Target (20h)',
                    data: new Array(userNames.length).fill(target),
                    type: 'line',
                    borderColor: '#007bff',
                    backgroundColor: 'transparent',
                    borderWidth: 2,
                    pointBackgroundColor: '#007bff',
                    pointBorderColor: '#fff',
                    pointBorderWidth: 2
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'top'
                },
                tooltip: {
                    mode: 'index',
                    intersect: false
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    max: Math.max(target + 5, Math.max(...weeklyHours) + 5),
                    ticks: {
                        callback: function(value) {
                            return value + 'h';
                        }
                    }
                }
            }
        }
    });
}

// Initialize WebSocket connection
function initializeWebSocket() {
    // Don't initialize if not on dashboard page
    if (!isDashboardPage()) {
        console.log('Not on dashboard page, skipping WebSocket initialization');
        return;
    }
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    ws = new WebSocket(wsUrl);
    
    ws.onopen = function() {
        console.log('WebSocket connected');
        showConnectionStatus('Connected', 'success');
    };
    
    ws.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            handleWebSocketMessage(data);
        } catch (error) {
            console.error('Error parsing WebSocket message:', error);
        }
    };
    
    ws.onclose = function(event) {
        console.log('WebSocket disconnected', event.code, event.reason);
        
        // Check if it's an authentication error (401)
        if (event.code === 1006 && event.reason === '') {
            showConnectionStatus('Authentication Required', 'warning');
            // Don't attempt to reconnect for auth errors
            return;
        }
        
        showConnectionStatus('Disconnected', 'danger');
        
        // Only attempt to reconnect if we're still on dashboard page and not auth error
        setTimeout(function() {
            if (isDashboardPage() && (!ws || ws.readyState === WebSocket.CLOSED)) {
                initializeWebSocket();
            }
        }, 5000);
    };
    
    ws.onerror = function(error) {
        console.error('WebSocket error:', error);
        showConnectionStatus('Connection Error', 'danger');
    };
}

// Handle WebSocket messages
function handleWebSocketMessage(data) {
    // Add validation and logging
    if (!data || typeof data !== 'object') {
        console.error('Invalid WebSocket message format:', data);
        return;
    }

    if (!data.type) {
        console.error('WebSocket message missing type:', data);
        return;
    }

    console.log("WebSocket message received:", data.type, data);

    switch (data.type) {
        case 'initial_data':
        case 'user_update':
            if (data.data) {
                updateDashboardData(data.data);
            } else {
                console.error('WebSocket message missing data field:', data);
            }
            break;
        case 'single_user_update':
            if (data.data && data.data.user) {
                updateSingleUser(data.data.user);
            } else {
                console.error('WebSocket single_user_update missing user data:', data);
            }
            break;
        case 'analytics_update':
            if (data.data) {
                updateAnalytics(data.data);
            } else {
                console.error('WebSocket analytics_update missing data:', data);
            }
            break;
        default:
            console.log('Unknown WebSocket message type:', data.type);
    }
}

// Update dashboard data
function updateDashboardData(data) {
    if (!window.dashboardData) {
        console.warn('Dashboard data not initialized, skipping update');
        return;
    }
    
    if (!data || typeof data !== 'object') {
        console.error('Invalid data passed to updateDashboardData:', data);
        return;
    }
    
    if (data.users && Array.isArray(data.users)) {
        window.dashboardData.users = data.users;
        updateUserTable(data.users);
        updateAnalyticsCards();
        updateCharts();
        console.log('Dashboard updated with', data.users.length, 'users');
    } else {
        console.warn('No valid users array in dashboard data update:', data);
    }
    
    // Handle analytics data if present
    if (data.analytics) {
        // Update analytics if needed
        console.log('Analytics data received:', data.analytics);
    }
}

// Update single user data efficiently
function updateSingleUser(user) {
    if (!window.dashboardData || !window.dashboardData.users) return;
    
    // Find and update the user in our data
    const userIndex = window.dashboardData.users.findIndex(u => u.user_id === user.user_id);
    if (userIndex !== -1) {
        window.dashboardData.users[userIndex] = user;
    } else {
        // Add new user if not found
        window.dashboardData.users.push(user);
    }
    
    // Update only the affected table row instead of rebuilding entire table
    updateSingleUserInTable(user);
    
    // Update analytics and charts with new data
    updateAnalyticsCards();
    updateCharts();
}

// Update user table
function updateUserTable(users) {
    if (!dataTable) return;
    
    dataTable.clear();
    
    const rows = users.map(user => [
        `<div class="d-flex align-items-center">
            <div class="status-indicator ${user.is_currently_working ? 'online' : 'offline'} me-2"></div>
            <strong>${user.name}</strong>
        </div>`,
        user.email,
        user.is_currently_working ? 
            '<span class="badge bg-success"><i class="fas fa-circle me-1"></i>Working</span>' :
            '<span class="badge bg-secondary"><i class="fas fa-circle me-1"></i>Offline</span>',
        `<div class="d-flex align-items-center">
            ${user.weekly_hours.toFixed(1)}h
            <div class="progress ms-2" style="width: 60px; height: 8px;">
                <div class="progress-bar ${user.weekly_hours >= 20 ? 'bg-success' : user.weekly_hours >= 10 ? 'bg-warning' : 'bg-danger'}" 
                     style="width: ${Math.min((user.weekly_hours / 20) * 100, 100)}%"></div>
            </div>
        </div>`,
        `${user.monthly_hours.toFixed(1)}h`,
        `${(user.total_working_time / 3600).toFixed(1)}h`,
        `<small class="text-muted">${new Date(user.last_activity).toLocaleDateString('en-US', {
            month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit'
        })}</small>`
    ]);
    
    dataTable.rows.add(rows);
    dataTable.draw();
}

// Update single user in DataTable efficiently
function updateSingleUserInTable(user) {
    if (!dataTable) return;
    
    // Find the row with this user by searching through all rows
    let rowToUpdate = null;
    dataTable.rows().every(function(index) {
        const rowData = this.data();
        // Check if this row belongs to our user (using email as identifier)
        if (rowData[1] === user.email) {
            rowToUpdate = this;
            return false; // Break the loop
        }
    });
    
    const newRowData = [
        `<div class="d-flex align-items-center">
            <div class="status-indicator ${user.is_currently_working ? 'online' : 'offline'} me-2"></div>
            <strong>${user.name}</strong>
        </div>`,
        user.email,
        user.is_currently_working ? 
            '<span class="badge bg-success"><i class="fas fa-circle me-1"></i>Working</span>' :
            '<span class="badge bg-secondary"><i class="fas fa-circle me-1"></i>Offline</span>',
        `<div class="d-flex align-items-center">
            ${user.weekly_hours.toFixed(1)}h
            <div class="progress ms-2" style="width: 60px; height: 8px;">
                <div class="progress-bar ${user.weekly_hours >= 20 ? 'bg-success' : user.weekly_hours >= 10 ? 'bg-warning' : 'bg-danger'}" 
                     style="width: ${Math.min((user.weekly_hours / 20) * 100, 100)}%"></div>
            </div>
        </div>`,
        `${user.monthly_hours.toFixed(1)}h`,
        `${(user.total_working_time / 3600).toFixed(1)}h`,
        `<small class="text-muted">${new Date(user.last_activity).toLocaleDateString('en-US', {
            month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit'
        })}</small>`
    ];
    
    if (rowToUpdate) {
        // Update existing row
        rowToUpdate.data(newRowData).draw(false);
    } else {
        // Add new row if user not found
        dataTable.row.add(newRowData).draw(false);
    }
}

// Update analytics cards
function updateAnalyticsCards() {
    const users = window.dashboardData.users || [];
    const activeUsers = users.filter(u => u.is_currently_working).length;
    const totalWeeklyHours = users.reduce((sum, u) => sum + u.weekly_hours, 0);
    const totalMonthlyHours = users.reduce((sum, u) => sum + u.monthly_hours, 0);
    
    $('#total-users').text(users.length);
    $('#active-users').text(activeUsers);
    $('#weekly-hours').text(totalWeeklyHours.toFixed(1));
    $('#monthly-hours').text(totalMonthlyHours.toFixed(1));
}

// Update charts
function updateCharts() {
    updateStatusChart();
    updateProgressChart();
}

// Update status chart
function updateStatusChart() {
    if (!statusChart) return;
    
    const users = window.dashboardData.users || [];
    const activeUsers = users.filter(u => u.is_currently_working).length;
    const inactiveUsers = users.length - activeUsers;
    
    statusChart.data.datasets[0].data = [activeUsers, inactiveUsers];
    statusChart.update();
}

// Update progress chart
function updateProgressChart() {
    if (!progressChart) return;
    
    const users = window.dashboardData.users || [];
    const topUsers = getTopPerformingUsers(users, 10, 10); // Min 10 hours, max 10 users
    
    const userNames = topUsers.map(u => u.name.split(' ')[0]);
    const weeklyHours = topUsers.map(u => u.weekly_hours);
    const target = 20;
    
    progressChart.data.labels = userNames;
    progressChart.data.datasets[0].data = weeklyHours;
    progressChart.data.datasets[0].backgroundColor = weeklyHours.map(hours => 
        hours >= target ? '#28a745' : 
        hours >= target * 0.5 ? '#ffc107' : '#dc3545'
    );
    progressChart.data.datasets[1].data = new Array(userNames.length).fill(target);
    progressChart.update();
}

// Update analytics data
function updateAnalytics(data) {
    if (!data || typeof data !== 'object') {
        console.error('Invalid analytics data:', data);
        return;
    }
    
    console.log('Updating analytics:', data);
    
    // Update users if provided
    if (data.users && Array.isArray(data.users)) {
        if (!window.dashboardData) {
            window.dashboardData = {};
        }
        window.dashboardData.users = data.users;
        updateUserTable(data.users);
        updateAnalyticsCards();
        updateCharts();
    }
    
    // Handle analytics-specific data
    if (data.analytics) {
        // Update any analytics-specific elements here
        console.log('Analytics stats:', data.analytics);
    }
}

// Show connection status
function showConnectionStatus(text, type) {
    const statusElement = $('#connection-status .alert');
    const textElement = $('#connection-text');
    
    statusElement.removeClass('alert-success alert-danger alert-warning')
                 .addClass(`alert-${type}`)
                 .show();
    textElement.text(text);
    
    // Auto-hide success messages after 3 seconds
    if (type === 'success') {
        setTimeout(() => {
            statusElement.fadeOut();
        }, 3000);
    }
}

// Refresh data manually
function refreshData() {
    $.ajax({
        url: '/api/users',
        method: 'GET',
        success: function(data) {
            updateDashboardData(data);
            showConnectionStatus('Data refreshed', 'success');
        },
        error: function() {
            showConnectionStatus('Failed to refresh data', 'danger');
        }
    });
}

// Export data
function exportData(type) {
    const exportUrl = `/api/export/excel?type=${type}`;
    
    // Create temporary link and click it
    const link = document.createElement('a');
    link.href = exportUrl;
    link.download = `time_tracker_${type}_${new Date().toISOString().split('T')[0]}.csv`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    
    showConnectionStatus(`${type.charAt(0).toUpperCase() + type.slice(1)} report exported`, 'success');
}

// Utility function to format duration
function formatDuration(seconds) {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    
    if (hours > 0) {
        return `${hours}h ${minutes}m`;
    } else {
        return `${minutes}m`;
    }
}

// Utility function to format date
function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
    });
}

// Handle window resize for charts
$(window).on('resize', function() {
    if (statusChart) statusChart.resize();
    if (progressChart) progressChart.resize();
});

// Global error handler
window.onerror = function(msg, url, lineNo, columnNo, error) {
    console.error('Global error:', msg, 'at', url, ':', lineNo);
    return false;
}; 