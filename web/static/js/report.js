// Get validator info from rules config
function getValidatorInfo(validatorName) {
    if (!window.RULES_CONFIG || !Array.isArray(window.RULES_CONFIG)) {
        return {
            title: validatorName,
            description: 'Rule validation failed.'
        };
    }
    
    for (const rule of window.RULES_CONFIG) {
        if (rule.validators) {
            for (const validator of rule.validators) {
                if (validator.name === validatorName) {
                    return {
                        title: validator.ui_title || validatorName,
                        description: validator.ui_description || 'Rule validation failed.'
                    };
                }
            }
        }
    }
    
    return {
        title: validatorName,
        description: 'Rule validation failed.'
    };
}

// Job navigation
function showJob(jobId) {
    document.querySelectorAll('.job-section').forEach(section => {
        section.classList.remove('active');
    });
    document.getElementById(jobId).classList.add('active');
    
    document.querySelectorAll('.job-item').forEach(item => {
        item.classList.remove('active');
    });
    document.querySelector('[data-job-id="' + jobId + '"]').classList.add('active');
    
    window.scrollTo(0, 0);
}

// Search functionality
document.addEventListener('DOMContentLoaded', () => {
    const searchBox = document.getElementById('searchBox');
    if (searchBox) {
        searchBox.addEventListener('input', (e) => {
            const searchTerm = e.target.value.toLowerCase();
            document.querySelectorAll('.job-item').forEach(item => {
                const jobName = item.querySelector('.job-item-name').textContent.toLowerCase();
                item.style.display = jobName.includes(searchTerm) ? 'block' : 'none';
            });
        });
    }
});

// Metric detail modal
function showMetricDetail(metricName, labels, cardinality, status, failedRulesStr, labelCardinalityJSON) {
    const panel = document.getElementById('metricDetailPanel');
    const overlay = document.getElementById('modalOverlay');
    const failedRules = failedRulesStr ? failedRulesStr.split('|').filter(r => r) : [];
    
    document.getElementById('metricDetailName').textContent = metricName;
    
    const statusHtml = status === 'pass' 
        ? '<span class="metric-status-badge metric-status-pass">✓ Pass</span>'
        : '<span class="metric-status-badge metric-status-fail">⚠ Failed</span>';
    document.getElementById('metricDetailStatus').innerHTML = statusHtml;
    
    const cardNum = parseInt(cardinality) || 0;
    document.getElementById('metricDetailCardinality').textContent = cardNum.toLocaleString();
    
    const labelsArray = labels ? labels.split(',').map(l => l.trim()).filter(l => l) : [];
    document.getElementById('metricDetailLabelCount').textContent = labelsArray.length;
    
    // Parse label cardinality JSON if available
    let labelCardinality = null;
    if (labelCardinalityJSON && labelCardinalityJSON !== '') {
        try {
            labelCardinality = JSON.parse(labelCardinalityJSON);
        } catch (e) {
            console.warn('Failed to parse label cardinality JSON:', e);
        }
    }
    
    const labelsContainer = document.getElementById('metricDetailLabels');
    if (labelsArray.length > 0) {
        let html = '';
        labelsArray.forEach((label) => {
            html += '<div class="metric-detail-info-row">';
            html += '<span class="metric-detail-info-label"><span class="metric-label-tag">' + label + '</span></span>';
            
            // Use actual cardinality if available, otherwise show estimate
            if (labelCardinality && labelCardinality[label] !== undefined) {
                html += '<span class="metric-detail-info-value" style="color: #4caf50; font-size: 11px;">' + labelCardinality[label].toLocaleString() + '</span>';
            } else {
                html += '<span class="metric-detail-info-value" style="color: #888; font-size: 11px;">~' + Math.ceil(cardNum / labelsArray.length).toLocaleString() + ' est.</span>';
            }
            
            html += '</div>';
        });
        labelsContainer.innerHTML = html;
        document.getElementById('metricLabelsSection').style.display = 'block';
        // Start collapsed by default
        document.getElementById('metricDetailLabels').style.display = 'none';
        document.getElementById('labelToggleIcon').textContent = '▶';
    } else {
        labelsContainer.innerHTML = '<div style="color: #888; font-size: 12px; padding: 12px; text-align: center;">No labels</div>';
    }
    
    if (status !== 'pass' && failedRules.length > 0) {
        document.getElementById('metricIssuesSection').style.display = 'block';
        const issuesHtml = failedRules.map(rule => {
            const validatorInfo = getValidatorInfo(rule);
            const issueText = '<strong>' + validatorInfo.title + ':</strong> ' + validatorInfo.description;
            return '<div style="margin-bottom: 10px; padding: 10px; background: rgba(244, 67, 54, 0.1); border-radius: 6px; font-size: 12px; color: #f44336;">' + issueText + '</div>';
        }).join('');
        document.getElementById('metricDetailIssues').innerHTML = issuesHtml;
        
        document.getElementById('metricRecommendationsSection').style.display = 'block';
        const recommendations = generateRecommendations(metricName, labelsArray, cardinality, failedRules);
        document.getElementById('metricDetailRecommendations').innerHTML = recommendations;
    } else {
        document.getElementById('metricIssuesSection').style.display = 'none';
        document.getElementById('metricRecommendationsSection').style.display = 'none';
    }
    
    overlay.classList.add('open');
    panel.classList.add('open');
    document.body.style.overflow = 'hidden';
}

function closeMetricDetail() {
    const panel = document.getElementById('metricDetailPanel');
    const overlay = document.getElementById('modalOverlay');
    panel.classList.remove('open');
    overlay.classList.remove('open');
    document.body.style.overflow = '';
}

function generateRecommendations(metricName, labels, cardinality, failedRules) {
    let recommendations = [];
    
    failedRules.forEach(rule => {
        if (rule.includes('prom_metrics_format_check')) {
            recommendations.push({
                title: 'Fix Naming Convention',
                text: 'Rename the metric to follow Prometheus standards: use snake_case and add appropriate suffix (_total for counters, _seconds for durations, _bytes for sizes, _ratio for ratios).',
                code: '# Example:\n' + metricName + ' → ' + metricName.toLowerCase().replace(/[^a-z0-9_]/g, '_') + '_total'
            });
        }
        
        if (rule.includes('prom_label_name_format_check')) {
            const invalidLabels = labels.filter(label => !label.match(/^[a-z][a-z0-9_]*$/));
            const fixedLabels = invalidLabels.map(label => label.toLowerCase().replace(/[^a-z0-9_]/g, '_'));
            recommendations.push({
                title: 'Fix Label Name Format',
                text: 'Label names must follow Prometheus conventions: lowercase, snake_case, starting with a letter. Invalid labels found: ' + invalidLabels.join(', '),
                code: '# Current (invalid labels):\n' + labels.join(', ') + '\n\n# Recommended:\n' + labels.map(l => l.toLowerCase().replace(/[^a-z0-9_]/g, '_')).join(', ')
            });
        }
        
        if (rule.includes('prom_metrics_cardinality_check')) {
            recommendations.push({
                title: 'Reduce Cardinality',
                text: 'High cardinality (>10,000) causes performance issues. Consider: (1) Remove high-cardinality labels, (2) Use label aggregation, (3) Sample data, or (4) Use recording rules.',
                code: '# Consider using recording rules:\n- record: ' + metricName + ':1m\n  expr: rate(' + metricName + '[1m])'
            });
        }
        
        if (rule.includes('prom_metrics_label_size_check')) {
            recommendations.push({
                title: 'Remove High-Cardinality Labels',
                text: 'Labels like user_id, session_id, request_id, and trace_id create unbounded cardinality. Use exemplars or separate logging systems for this data instead.',
                code: '# Bad:\n' + metricName + '{user_id="123", ...}\n\n# Good:\n' + metricName + '{service="api", ...}'
            });
        }
        
        if (rule.includes('prom_metrics_label_count_check')) {
            const labelCount = labels.length;
            recommendations.push({
                title: 'Reduce Label Count (' + labelCount + ' labels)',
                text: 'Having ' + labelCount + ' labels increases cardinality risk exponentially. Keep only essential dimensions. Many Kubernetes labels (pod, container, endpoint) are redundant.',
                code: '# Current (' + labelCount + ' labels):\n' + labels.join(', ') + '\n\n# Recommended (3-5 core labels):\nservice, environment, region'
            });
        }
    });
    
    return recommendations.map(rec => 
        '<div class="metric-recommendation">' +
            '<div class="metric-recommendation-title">' + rec.title + '</div>' +
            '<div class="metric-recommendation-text">' + rec.text + '</div>' +
            (rec.code ? '<div class="metric-recommendation-code">' + rec.code + '</div>' : '') +
        '</div>'
    ).join('');
}

// Table sorting
let sortDirections = {};

function sortTable(jobIndex, columnIndex) {
    const table = document.getElementById('metrics-table-' + jobIndex);
    const tbody = table.querySelector('tbody');
    const rows = Array.from(tbody.querySelectorAll('tr'));
    
    const sortKey = jobIndex + '-' + columnIndex;
    const ascending = sortDirections[sortKey] !== true;
    sortDirections[sortKey] = ascending;
    
    rows.sort((a, b) => {
        let aValue, bValue;
        
        if (columnIndex === 2) {
            aValue = parseInt(a.cells[columnIndex].getAttribute('data-value') || '0');
            bValue = parseInt(b.cells[columnIndex].getAttribute('data-value') || '0');
        } else if (columnIndex === 3) {
            aValue = a.cells[columnIndex].getAttribute('data-status');
            bValue = b.cells[columnIndex].getAttribute('data-status');
        } else {
            aValue = a.cells[columnIndex].textContent.trim().toLowerCase();
            bValue = b.cells[columnIndex].textContent.trim().toLowerCase();
        }
        
        if (aValue < bValue) return ascending ? -1 : 1;
        if (aValue > bValue) return ascending ? 1 : -1;
        return 0;
    });
    
    rows.forEach(row => tbody.appendChild(row));
    
    const headers = table.querySelectorAll('th');
    headers.forEach((header, idx) => {
        if (idx === columnIndex) {
            header.textContent = header.textContent.split(' ')[0] + ' ' + (ascending ? '▲' : '▼');
        } else {
            const text = header.textContent.split(' ')[0];
            header.textContent = text + ' ▼';
        }
    });
}

// Keyboard support
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && document.getElementById('modalOverlay').classList.contains('open')) {
        closeMetricDetail();
    }
});

// Toggle label breakdown collapse
function toggleLabelBreakdown() {
    const labelsContainer = document.getElementById('metricDetailLabels');
    const icon = document.getElementById('labelToggleIcon');
    
    if (labelsContainer.style.display === 'none') {
        labelsContainer.style.display = 'block';
        icon.textContent = '▼';
    } else {
        labelsContainer.style.display = 'none';
        icon.textContent = '▶';
    }
}

// Get rule description from config
function getRuleDescription(ruleID) {
    if (!window.RULES_CONFIG || !Array.isArray(window.RULES_CONFIG)) {
        return 'No description available.';
    }
    
    for (const rule of window.RULES_CONFIG) {
        if (rule.rule_id === ruleID) {
            return rule.description || 'No description available.';
        }
    }
    return 'No description available.';
}

// New wrapper function to read data from card and calculate total denominator
function showRuleDetailFromCard(cardElement, jobName) {
    // Get this rule's data
    const ruleID = cardElement.dataset.ruleId;
    const passedMetrics = parseInt(cardElement.dataset.passedMetrics);
    const totalMetrics = parseInt(cardElement.dataset.totalMetrics);
    const passedCardinality = parseInt(cardElement.dataset.passedCardinality);
    const totalCardinality = parseInt(cardElement.dataset.totalCardinality);
    const impact = cardElement.dataset.impact;
    
    // Get all rules for this job to calculate total denominator
    const rulesContainer = cardElement.parentElement;
    const allRuleCards = rulesContainer.querySelectorAll('.rule-card');
    
    let totalDenominator = 0;
    const impactWeights = {
        'Critical': 40,
        'Important': 30,
        'Normal': 20,
        'Low': 10
    };
    
    // Calculate total denominator and numerator across all rules (match backend logic)
    let totalNumerator = 0;
    allRuleCards.forEach(card => {
        const cardPassedCardinality = parseInt(card.dataset.passedCardinality);
        const cardTotalCardinality = parseInt(card.dataset.totalCardinality);
        const cardPassedMetrics = parseInt(card.dataset.passedMetrics);
        const cardTotalMetrics = parseInt(card.dataset.totalMetrics);
        const cardImpact = card.dataset.impact;
        const weight = impactWeights[cardImpact] || 20;
        
        // Backend uses: if (result.TotalCardinality > 0) for cardinality-weighted scoring
        // Rules using "cardinality" data source have TotalCardinality > 0
        // Rules using "labels" data source have TotalCardinality = 0
        if (cardTotalCardinality > 0) {
            totalNumerator += cardPassedCardinality * weight;
            totalDenominator += cardTotalCardinality * weight;
        } else {
            totalNumerator += cardPassedMetrics * weight;
            totalDenominator += cardTotalMetrics * weight;
        }
    });
    
    // Calculate final score
    const finalScore = (totalNumerator / totalDenominator) * 100;
    
    // Call the actual modal function with total denominator and final score
    showRuleDetail(jobName, ruleID, passedMetrics, totalMetrics, passedCardinality, totalCardinality, impact, totalDenominator, finalScore);
}

// Show rule detail modal
function showRuleDetail(jobName, ruleID, passedMetrics, totalMetrics, passedCardinality, totalCardinality, impact, totalDenominator, finalScore) {
    const impactWeights = {
        'Critical': 40,
        'Important': 30,
        'Normal': 20,
        'Low': 10
    };
    
    const weight = impactWeights[impact] || 20;
    const passRatePercent = ((passedMetrics / totalMetrics) * 100).toFixed(1);
    
    // Determine if this rule uses cardinality-weighted scoring (match backend logic)
    // Backend uses: if (result.TotalCardinality > 0) for cardinality-weighted scoring
    // Rules using "cardinality" data source have TotalCardinality > 0
    // Rules using "labels" data source have TotalCardinality = 0
    const usesCardinalityScoring = totalCardinality > 0;
    
    // Calculate this rule's numerator
    let ruleNumerator;
    
    if (usesCardinalityScoring) {
        ruleNumerator = passedCardinality * weight;
    } else {
        ruleNumerator = passedMetrics * weight;
    }
    
    // Calculate points earned and points possible
    const pointsEarned = ruleNumerator;
    const pointsPossible = (usesCardinalityScoring ? totalCardinality : totalMetrics) * weight;
    
    // Calculate this rule's absolute contribution to score
    const absoluteContribution = (ruleNumerator / totalDenominator) * 100;
    
    // Calculate this rule's percentage of the final score
    const percentageOfFinalScore = (absoluteContribution / finalScore) * 100;
    
    const impactColor = impact === 'Critical' ? '#f44336' : 
                       impact === 'Important' ? '#ff9800' : 
                       impact === 'Normal' ? '#2196f3' : '#9e9e9e';
    
    // Update modal
    document.getElementById('ruleDetailTitle').innerHTML = `${ruleID} <span style="color: #888; font-weight: normal; font-size: 16px;">- ${jobName}</span>`;
    
    // Rule description
    document.getElementById('ruleDescription').textContent = getRuleDescription(ruleID);
    
    // Contribution - show what percentage of the final score this rule represents
    const contributionColor = percentageOfFinalScore > 50 ? '#4caf50' : percentageOfFinalScore > 25 ? '#8bc34a' : percentageOfFinalScore > 10 ? '#ff9800' : '#f44336';
    document.getElementById('ruleContribution').innerHTML = `
        <div style="font-size: 24px; font-weight: bold; color: ${contributionColor};">${percentageOfFinalScore.toFixed(1)}%</div>
        <div style="font-size: 11px; color: #888; margin-top: 4px;">of final score</div>
    `;
    
    // Points Earned and Points Possible
    document.getElementById('rulePointsEarned').textContent = pointsEarned.toLocaleString();
    document.getElementById('rulePointsPossible').textContent = pointsPossible.toLocaleString();
    
    // Metrics Passed
    document.getElementById('ruleMetricsPassed').innerHTML = `
        <div style="font-size: 24px; font-weight: bold; color: ${passedMetrics === totalMetrics ? '#4caf50' : '#ff9800'};">${passedMetrics}/${totalMetrics}</div>
        <div style="font-size: 11px; color: #888; margin-top: 4px;">metrics</div>
    `;
    
    // Pass Rate
    document.getElementById('rulePassRate').innerHTML = `
        <div style="font-size: 24px; font-weight: bold; color: ${parseFloat(passRatePercent) >= 90 ? '#4caf50' : parseFloat(passRatePercent) >= 75 ? '#8bc34a' : parseFloat(passRatePercent) >= 50 ? '#ff9800' : '#f44336'};">${passRatePercent}%</div>
        <div style="font-size: 11px; color: #888; margin-top: 4px;">of metrics</div>
    `;
    
    // Impact Level
    document.getElementById('ruleImpactLevel').innerHTML = `
        <div style="font-size: 20px; font-weight: 600; color: ${impactColor};">${impact}</div>
        <div style="font-size: 11px; color: #888; margin-top: 4px;">weight: ${weight}</div>
    `;
    
    // Cardinality section - only show for rules that use cardinality-weighted scoring
    if (usesCardinalityScoring) {
        document.getElementById('ruleCardinalitySection').style.display = 'block';
        document.getElementById('rulePassedCardinality').textContent = passedCardinality.toLocaleString();
        document.getElementById('ruleTotalCardinality').textContent = totalCardinality.toLocaleString();
    } else {
        document.getElementById('ruleCardinalitySection').style.display = 'none';
    }
    
    // Detailed calculation breakdown
    const maxContribution = (pointsPossible / totalDenominator) * 100;
    const lostScore = maxContribution - absoluteContribution;
    
    let explanationHTML = '';
    if (usesCardinalityScoring) {
        const cardinalityPercent = ((passedCardinality / totalCardinality) * 100).toFixed(1);
        const failedSeries = totalCardinality - passedCardinality;
        explanationHTML = `
            <div style="color: #bbb; line-height: 1.8;">
                <strong style="color: #fff;">Calculation Breakdown:</strong><br>
                <div style="margin: 15px 0; padding: 15px; background: rgba(255,255,255,0.03); border-radius: 8px; font-family: monospace; font-size: 12px;">
                    <div style="margin-bottom: 10px;">
                        <strong style="color: #4a9eff;">Step 1: Calculate Points</strong><br>
                        Points Earned = PassedCardinality × Weight<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= ${passedCardinality.toLocaleString()} × ${weight}<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #4caf50;">${pointsEarned.toLocaleString()}</strong><br>
                        <br>
                        Points Possible = TotalCardinality × Weight<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= ${totalCardinality.toLocaleString()} × ${weight}<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #888;">${pointsPossible.toLocaleString()}</strong>
                    </div>
                    <div style="margin-bottom: 10px;">
                        <strong style="color: #4a9eff;">Step 2: Calculate Contribution</strong><br>
                        Absolute Contribution = (Points Earned / Total Denominator) × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= (${pointsEarned.toLocaleString()} / ${totalDenominator.toLocaleString()}) × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #4caf50;">${absoluteContribution.toFixed(3)}%</strong><br>
                        <br>
                        % of Final Score (${finalScore.toFixed(2)}%) = (${absoluteContribution.toFixed(3)}% / ${finalScore.toFixed(2)}%) × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #4a9eff;">${percentageOfFinalScore.toFixed(1)}%</strong>
                    </div>
                    <div>
                        <strong style="color: #4a9eff;">Step 3: Calculate Lost Score</strong><br>
                        Lost Score = (Points Possible - Points Earned) / Total Denominator × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= (${pointsPossible.toLocaleString()} - ${pointsEarned.toLocaleString()}) / ${totalDenominator.toLocaleString()} × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #f44336;">${lostScore.toFixed(3)}%</strong>
                    </div>
                </div>
                <div style="margin: 15px 0;">
                    This rule evaluates <strong style="color: #fff;">${totalCardinality.toLocaleString()} time series</strong> across ${totalMetrics} metrics.<br>
                    <strong style="color: #4caf50;">${passedCardinality.toLocaleString()} series passed</strong> (${cardinalityPercent}%), 
                    <strong style="color: #f44336;">${failedSeries.toLocaleString()} series failed</strong>.
                </div>
                <div style="padding: 10px; background: rgba(255,152,0,0.1); border-radius: 6px; font-size: 12px; color: #ff9800;">
                    ⚡ Cardinality-weighted: Each series counts individually toward the score
                </div>
            </div>
        `;
    } else {
        const failedMetrics = totalMetrics - passedMetrics;
        explanationHTML = `
            <div style="color: #bbb; line-height: 1.8;">
                <strong style="color: #fff;">Calculation Breakdown:</strong><br>
                <div style="margin: 15px 0; padding: 15px; background: rgba(255,255,255,0.03); border-radius: 8px; font-family: monospace; font-size: 12px;">
                    <div style="margin-bottom: 10px;">
                        <strong style="color: #4a9eff;">Step 1: Calculate Points</strong><br>
                        Points Earned = PassedMetrics × Weight<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= ${passedMetrics} × ${weight}<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #4caf50;">${pointsEarned.toLocaleString()}</strong><br>
                        <br>
                        Points Possible = TotalMetrics × Weight<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= ${totalMetrics} × ${weight}<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #888;">${pointsPossible.toLocaleString()}</strong>
                    </div>
                    <div style="margin-bottom: 10px;">
                        <strong style="color: #4a9eff;">Step 2: Calculate Contribution</strong><br>
                        Absolute Contribution = (Points Earned / Total Denominator) × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= (${pointsEarned.toLocaleString()} / ${totalDenominator.toLocaleString()}) × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #4caf50;">${absoluteContribution.toFixed(3)}%</strong><br>
                        <br>
                        % of Final Score (${finalScore.toFixed(2)}%) = (${absoluteContribution.toFixed(3)}% / ${finalScore.toFixed(2)}%) × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #4a9eff;">${percentageOfFinalScore.toFixed(1)}%</strong>
                    </div>
                    <div>
                        <strong style="color: #4a9eff;">Step 3: Calculate Lost Score</strong><br>
                        Lost Score = (Points Possible - Points Earned) / Total Denominator × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= (${pointsPossible.toLocaleString()} - ${pointsEarned.toLocaleString()}) / ${totalDenominator.toLocaleString()} × 100<br>
                        &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;= <strong style="color: #f44336;">${lostScore.toFixed(3)}%</strong>
                    </div>
                </div>
                <div style="margin: 15px 0;">
                    This rule evaluates <strong style="color: #fff;">${totalMetrics} metrics</strong>.<br>
                    <strong style="color: #4caf50;">${passedMetrics} metrics passed</strong> (${passRatePercent}%), 
                    <strong style="color: #f44336;">${failedMetrics} metrics failed</strong>.
                </div>
            </div>
        `;
    }
    document.getElementById('ruleScoreBreakdown').innerHTML = explanationHTML;
    
    // Show modal with same pattern as metric detail
    document.getElementById('ruleDetailModal').classList.add('open');
    document.getElementById('ruleDetailPanel').classList.add('open');
    document.body.style.overflow = 'hidden';
}

// Close rule detail modal
function closeRuleDetail() {
    document.getElementById('ruleDetailModal').classList.remove('open');
    document.getElementById('ruleDetailPanel').classList.remove('open');
    document.body.style.overflow = '';
}

