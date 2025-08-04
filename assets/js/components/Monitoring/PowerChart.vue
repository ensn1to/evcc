<template>
	<div class="power-chart">
		<canvas ref="chartCanvas" :width="chartWidth" :height="chartHeight"></canvas>
	</div>
</template>

<script>
import { Chart, registerables } from 'chart.js';

Chart.register(...registerables);

export default {
	name: "PowerChart",
	props: {
		data: {
			type: Array,
			default: () => [],
		},
		dataType: {
			type: String,
			default: "sitepower",
		},
		selectedDate: {
			type: String,
			default: "",
		},
	},
	data() {
		return {
			chart: null,
			chartWidth: 800,
			chartHeight: 400,
		};
	},
	computed: {
		chartData() {
			return {
				labels: this.data.map(point => {
					const date = new Date(point.time);
					return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
				}),
				datasets: [{
					label: this.dataType === 'sitepower' ? 'Site Power (kW)' : 'Battery Power (kW)',
					data: this.data.map(point => point.power),
					borderColor: this.dataType === 'sitepower' ? '#007bff' : '#28a745',
					backgroundColor: this.dataType === 'sitepower' ? 'rgba(0, 123, 255, 0.1)' : 'rgba(40, 167, 69, 0.1)',
					borderWidth: 2,
					fill: true,
					tension: 0.4,
				}],
			};
		},
		chartOptions() {
			return {
				responsive: true,
				maintainAspectRatio: false,
				scales: {
					x: {
						title: {
							display: true,
							text: 'Time',
						},
						grid: {
							display: true,
							color: 'rgba(0, 0, 0, 0.1)',
						},
					},
					y: {
						title: {
							display: true,
							text: 'Power (kW)',
						},
						grid: {
							display: true,
							color: 'rgba(0, 0, 0, 0.1)',
						},
						beginAtZero: true,
					},
				},
				plugins: {
					legend: {
						display: true,
						position: 'top',
					},
					tooltip: {
						mode: 'index',
						intersect: false,
						callbacks: {
							label: (context) => {
								return `${context.dataset.label}: ${context.parsed.y.toFixed(2)} kW`;
							},
						},
					},
				},
				interaction: {
					mode: 'nearest',
					axis: 'x',
					intersect: false,
				},
			};
		},
	},
	mounted() {
		this.initChart();
		this.handleResize();
		window.addEventListener('resize', this.handleResize);
	},
	beforeUnmount() {
		if (this.chart) {
			this.chart.destroy();
		}
		window.removeEventListener('resize', this.handleResize);
	},
	watch: {
		data: {
			handler() {
				this.updateChart();
			},
			deep: true,
		},
		dataType() {
			this.updateChart();
		},
	},
	methods: {
		initChart() {
			const ctx = this.$refs.chartCanvas.getContext('2d');
			this.chart = new Chart(ctx, {
				type: 'line',
				data: this.chartData,
				options: this.chartOptions,
			});
		},
		updateChart() {
			if (this.chart) {
				this.chart.data = this.chartData;
				this.chart.update();
			}
		},
		handleResize() {
			const container = this.$el;
			if (container) {
				this.chartWidth = container.clientWidth;
				this.chartHeight = Math.min(400, container.clientWidth * 0.5);
			}
		},
	},
};
</script>

<style scoped>
.power-chart {
	position: relative;
	width: 100%;
	height: 400px;
	margin: 1rem 0;
}

canvas {
	max-width: 100%;
	height: auto;
}
</style>