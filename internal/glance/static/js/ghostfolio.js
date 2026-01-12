export default function(element) {
    const container = element.querySelector('.ghostfolio-container');
    const rangeButtons = element.querySelectorAll('.ghostfolio-range-btn');
    
    if (!container || rangeButtons.length === 0) return;

    // Parse preloaded portfolio data from data attribute
    let portfolioData;
    try {
        portfolioData = JSON.parse(element.dataset.portfolio || '{}');
    } catch (e) {
        console.error('Failed to parse Ghostfolio portfolio data:', e);
        return;
    }

    const chartType = element.dataset.chartType || 'value';
    let currentRange = element.dataset.defaultRange || 'max';

    const currencyEl = element.querySelector('[data-currency]');
    const amountEl = element.querySelector('[data-amount]');
    const performanceEl = element.querySelector('[data-performance]');
    const perfValueEl = element.querySelector('[data-perf-value]');
    const perfPercentEl = element.querySelector('[data-perf-percent]');
    const polygonEl = element.querySelector('[data-polygon]');
    const polylineEl = element.querySelector('[data-polyline]');
    const gradientStartEl = element.querySelector('[data-gradient-start]');
    const gradientEndEl = element.querySelector('[data-gradient-end]');
    
    // Hover elements
    const chartEl = element.querySelector('[data-chart]');
    const hoverLineEl = element.querySelector('[data-hover-line]');
    const hoverDotEl = element.querySelector('[data-hover-dot]');
    const tooltipEl = element.querySelector('[data-tooltip]');
    const tooltipDateEl = element.querySelector('[data-tooltip-date]');
    const tooltipValueEl = element.querySelector('[data-tooltip-value]');
    const tooltipPerfEl = element.querySelector('[data-tooltip-perf]');

    function formatPrice(value) {
        return value.toLocaleString('en-US', {
            minimumFractionDigits: 2,
            maximumFractionDigits: 2
        });
    }

    function formatDate(dateStr) {
        const date = new Date(dateStr);
        return date.toLocaleDateString('fr-FR', {
            day: '2-digit',
            month: 'short',
            year: 'numeric'
        });
    }

    function getChartPoints(data) {
        if (chartType === 'performance') {
            return data.performanceChartPoints || data.chartPoints;
        }
        return data.chartPoints;
    }

    function updateUI(data) {
        if (!data || !data.hasData) return;

        const isPositive = data.netPerformancePct >= 0;

        // Update currency and amount
        if (currencyEl) currencyEl.textContent = data.currency;
        if (amountEl) amountEl.textContent = formatPrice(data.totalValue);

        // Update performance colors
        if (performanceEl) {
            performanceEl.classList.remove('color-positive', 'color-negative');
            performanceEl.classList.add(isPositive ? 'color-positive' : 'color-negative');
        }

        // Update performance values
        const sign = isPositive ? '+' : '';
        if (perfValueEl) perfValueEl.textContent = `${sign}${data.currency}${formatPrice(data.netPerformance)}`;
        if (perfPercentEl) perfPercentEl.textContent = `(${data.netPerformancePct >= 0 ? '+' : ''}${data.netPerformancePct.toFixed(2)}%)`;

        // Update chart
        const chartPoints = getChartPoints(data);
        if (chartPoints && polygonEl && polylineEl) {
            polygonEl.setAttribute('points', `0,50 ${chartPoints} 100,50`);
            polylineEl.setAttribute('points', chartPoints);
            
            polylineEl.classList.remove('positive', 'negative');
            polylineEl.classList.add(isPositive ? 'positive' : 'negative');

            // Update gradient classes
            if (gradientStartEl) {
                gradientStartEl.classList.remove('positive', 'negative');
                gradientStartEl.classList.add(isPositive ? 'positive' : 'negative');
            }
            if (gradientEndEl) {
                gradientEndEl.classList.remove('positive', 'negative');
                gradientEndEl.classList.add(isPositive ? 'positive' : 'negative');
            }
        }
    }

    function switchRange(range) {
        const data = portfolioData[range];
        if (!data) {
            console.warn(`No data available for range: ${range}`);
            return;
        }

        currentRange = range;
        updateUI(data);

        // Update active button
        rangeButtons.forEach(btn => {
            btn.classList.toggle('active', btn.dataset.range === range);
        });
    }

    // Hover handling for tooltip
    function handleChartHover(e) {
        const data = portfolioData[currentRange];
        if (!data || !data.chartData || data.chartData.length === 0) return;

        const rect = chartEl.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const relativeX = x / rect.width;
        
        // Find the data point index based on position
        const index = Math.min(
            Math.max(0, Math.round(relativeX * (data.chartData.length - 1))),
            data.chartData.length - 1
        );
        
        const point = data.chartData[index];
        if (!point) return;

        // Calculate SVG coordinates
        const svgX = (index / (data.chartData.length - 1)) * 100;
        
        // Get Y value from the polyline points
        const chartPoints = getChartPoints(data);
        const pointsArray = chartPoints.split(' ').map(p => {
            const [px, py] = p.split(',');
            return { x: parseFloat(px), y: parseFloat(py) };
        });
        
        // Find closest point in the polyline
        let closestPoint = pointsArray[0];
        let minDist = Math.abs(pointsArray[0].x - svgX);
        for (const p of pointsArray) {
            const dist = Math.abs(p.x - svgX);
            if (dist < minDist) {
                minDist = dist;
                closestPoint = p;
            }
        }

        // Update hover line position
        if (hoverLineEl) {
            hoverLineEl.setAttribute('x1', closestPoint.x);
            hoverLineEl.setAttribute('x2', closestPoint.x);
        }

        // Update hover dot position (convert SVG coords to pixels)
        if (hoverDotEl) {
            const dotX = (closestPoint.x / 100) * rect.width;
            const dotY = (closestPoint.y / 50) * rect.height;
            hoverDotEl.style.left = `${dotX}px`;
            hoverDotEl.style.top = `${dotY}px`;
        }

        // Update tooltip content
        if (tooltipDateEl) tooltipDateEl.textContent = formatDate(point.date);
        if (tooltipValueEl) tooltipValueEl.textContent = `${data.currency}${formatPrice(point.value)}`;
        if (tooltipPerfEl) {
            const isPositive = point.performancePct >= 0;
            tooltipPerfEl.textContent = `${isPositive ? '+' : ''}${point.performancePct.toFixed(2)}%`;
            tooltipPerfEl.classList.remove('positive', 'negative');
            tooltipPerfEl.classList.add(isPositive ? 'positive' : 'negative');
        }

        // Position tooltip
        if (tooltipEl) {
            const tooltipWidth = tooltipEl.offsetWidth;
            const tooltipHeight = tooltipEl.offsetHeight;
            
            // Calculate position relative to chart container
            let tooltipX = x - tooltipWidth / 2;
            let tooltipY = -tooltipHeight - 8;
            
            // Keep tooltip within bounds
            if (tooltipX < 0) tooltipX = 0;
            if (tooltipX + tooltipWidth > rect.width) tooltipX = rect.width - tooltipWidth;
            
            tooltipEl.style.transform = `translate(${tooltipX}px, ${tooltipY}px)`;
        }
    }

    // Add hover event listener for tooltip
    if (chartEl) {
        chartEl.addEventListener('mousemove', handleChartHover);
    }

    // Add click handlers to range buttons
    rangeButtons.forEach(button => {
        button.addEventListener('click', () => {
            if (!button.classList.contains('active')) {
                switchRange(button.dataset.range);
            }
        });
    });
}
