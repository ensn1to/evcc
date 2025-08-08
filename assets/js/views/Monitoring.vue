<template>
	<div class="container px-4 safe-area-inset">
		<TopHeader :title="$t('monitoring.title')" />
		<div class="row">
			<main class="col-12">
				<div class="header-outer sticky-top">
					<div class="container px-4">
						<div class="row py-3 py-sm-3 d-flex flex-column flex-sm-row gap-3 gap-lg-0 mb-lg-2">
							<div class="col-lg-5 d-flex mb-lg-0">
								<SelectGroup
									id="monitoringDataType"
									class="w-100"
									:options="dataTypeOptions"
									:modelValue="selectedDataType"
									@update:model-value="updateDataType"
								/>
							</div>
							<div class="col-lg-6 offset-lg-1">
								<DateSelector
									:selectedDate="selectedDate"
									@update-date="updateDate"
								/>
							</div>
						</div>
					</div>
				</div>

				<h3 class="fw-normal my-4 d-flex gap-3 flex-wrap d-flex align-items-baseline overflow-hidden">
					<span class="d-block no-wrap text-truncate">
						{{ chartTitle }}
					</span>
					<small class="d-block no-wrap text-truncate">{{ chartSubTitle }}</small>
				</h3>

				<div v-if="loading" class="d-flex justify-content-center my-5">
					<div class="spinner-border" role="status">
						<span class="visually-hidden">{{ $t('monitoring.loading') }}</span>
					</div>
				</div>

				<div v-else-if="!hasData" class="mb-5">
					<div class="table-responsive">
						<table class="table table-striped">
							<thead>
								<tr>
									<th scope="col">{{ $t('monitoring.table.time') }}</th>
									<th scope="col">{{ $t('monitoring.table.power') }}</th>
								</tr>
							</thead>
							<tbody>
								<tr>
									<td colspan="2" class="text-center text-muted py-4">
										{{ $t('monitoring.noData') }}
									</td>
								</tr>
							</tbody>
						</table>
					</div>
				</div>

				<div v-else class="mb-5">
					<PowerChart
						:data="chartData"
						:data-type="selectedDataType"
						:selected-date="selectedDate"
					/>
				</div>

				<div v-if="hasData" class="row">
					<div class="col-12">
						<h4 class="fw-normal mb-3">{{ $t('monitoring.statistics') }}</h4>
						<div class="row">
							<div class="col-md-3 col-6 mb-3">
								<div class="card text-center">
									<div class="card-body">
										<h5 class="card-title">{{ $t('monitoring.stats.max') }}</h5>
										<p class="card-text">{{ formatPower(maxPower) }}</p>
									</div>
								</div>
							</div>
							<div class="col-md-3 col-6 mb-3">
								<div class="card text-center">
									<div class="card-body">
										<h5 class="card-title">{{ $t('monitoring.stats.min') }}</h5>
										<p class="card-text">{{ formatPower(minPower) }}</p>
									</div>
								</div>
							</div>
							<div class="col-md-3 col-6 mb-3">
								<div class="card text-center">
									<div class="card-body">
										<h5 class="card-title">{{ $t('monitoring.stats.avg') }}</h5>
										<p class="card-text">{{ formatPower(avgPower) }}</p>
									</div>
								</div>
							</div>
							<div class="col-md-3 col-6 mb-3">
								<div class="card text-center">
									<div class="card-body">
										<h5 class="card-title">{{ $t('monitoring.stats.total') }}</h5>
										<p class="card-text">{{ dataPoints.length }}</p>
									</div>
								</div>
							</div>
						</div>
					</div>
				</div>
			</main>
		</div>
	</div>
</template>

<script>
import api from "../api";
import Header from "../components/Top/Header.vue";
import PowerChart from "../components/Monitoring/PowerChart.vue";
import SelectGroup from "../components/Helper/SelectGroup.vue";
import DateSelector from "../components/Monitoring/DateSelector.vue";
import formatter from "../mixins/formatter";
import store from "../store";

export default {
	name: "Monitoring",
	components: {
		TopHeader: Header,
		PowerChart,
		SelectGroup,
		DateSelector,
	},
	mixins: [formatter],
	props: {
		selectedDate: {
			type: String,
			default: () => new Date().toISOString().split('T')[0],
		},
		selectedDataType: {
			type: String,
			default: "sitepower",
		},
	},
	data() {
		return {
			dataPoints: [],
			loading: false,
			dataTypeOptions: [
				{ value: "sitepower", name: this.$t('monitoring.dataType.sitepower') },
				{ value: "battery", name: this.$t('monitoring.dataType.battery') },
			],
		};
	},
	head() {
		return { title: this.$t("monitoring.title") };
	},
	computed: {
		siteTitle() {
			return store.state.site?.title || "evcc";
		},
		chartTitle() {
			if (this.selectedDataType === "sitepower") {
				return this.$t("monitoring.chartTitle.sitepower");
			}
			return this.$t("monitoring.chartTitle.battery");
		},
		chartSubTitle() {
			const date = new Date(this.selectedDate);
			return date.toLocaleDateString();
		},
		hasData() {
			return this.dataPoints.length > 0;
		},
		chartData() {
			return this.dataPoints.map(point => ({
				time: point.createdAt,
				power: point.powerKW,
			}));
		},
		maxPower() {
			if (!this.hasData) return 0;
			return Math.max(...this.dataPoints.map(p => p.powerKW));
		},
		minPower() {
			if (!this.hasData) return 0;
			return Math.min(...this.dataPoints.map(p => p.powerKW));
		},
		avgPower() {
			if (!this.hasData) return 0;
			const sum = this.dataPoints.reduce((acc, p) => acc + p.powerKW, 0);
			return sum / this.dataPoints.length;
		},
	},
	mounted() {
		this.loadData();
	},
	watch: {
		selectedDate() {
			this.loadData();
		},
		selectedDataType() {
			this.loadData();
		},
	},
	methods: {
		async loadData() {
			this.loading = true;
			try {
				const date = new Date(this.selectedDate);
				const from = new Date(date.getFullYear(), date.getMonth(), date.getDate());
				const to = new Date(date.getFullYear(), date.getMonth(), date.getDate() + 1);
				
				// 确保使用正确的站点标题
				const siteTitle = store.state.siteTitle || store.state.site?.title || "evcc";
				
				const params = new URLSearchParams({
					site: siteTitle,
					from: from.toISOString(),
					to: to.toISOString(),
				});
				
				const response = await api.get(`sitepower/records?${params.toString()}`);
				this.dataPoints = response.data?.records || [];
			} catch (error) {
				console.error("Failed to load monitoring data:", error);
				this.dataPoints = [];
			} finally {
				this.loading = false;
			}
		},
		updateDataType(newDataType) {
			this.$router.push({ 
				query: { 
					...this.$route.query, 
					dataType: newDataType 
				}
			});
		},
		updateDate(newDate) {
			this.$router.push({ 
				query: { 
					...this.$route.query, 
					date: newDate 
				}
			});
		},
		formatPower(value) {
			return `${value.toFixed(2)} kW`;
		},
	},
};
</script>

<style scoped>
.header-outer {
	background-color: var(--evcc-background);
	z-index: 1020;
	border-bottom: 1px solid var(--evcc-box-border);
}

.card {
	border: 1px solid var(--evcc-box-border);
	border-radius: 12px;
	background-color: var(--evcc-background);
	box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	transition: box-shadow 0.15s ease-in-out;
}

.card:hover {
	box-shadow: 0 4px 8px rgba(0, 0, 0, 0.15);
}

.card-body {
	padding: 1.25rem;
}

.card-title {
	font-size: 0.875rem;
	font-weight: 600;
	color: var(--evcc-gray);
	margin-bottom: 0.5rem;
	text-transform: uppercase;
	letter-spacing: 0.025em;
}

.card-text {
	font-size: 1.5rem;
	font-weight: 700;
	color: var(--evcc-default-text);
	margin-bottom: 0;
}

.table {
	border: 1px solid var(--evcc-box-border);
	border-radius: 12px;
	overflow: hidden;
	background-color: var(--evcc-background);
}

.table th {
	background-color: var(--evcc-gray-light);
	border-bottom: 1px solid var(--evcc-box-border);
	font-weight: 600;
	font-size: 0.875rem;
	color: var(--evcc-default-text);
	padding: 1rem;
}

.table td {
	border-bottom: 1px solid var(--evcc-box-border);
	padding: 1rem;
	color: var(--evcc-default-text);
}

.table-striped > tbody > tr:nth-of-type(odd) > td {
	background-color: var(--evcc-gray-light);
}

.no-wrap {
	white-space: nowrap;
}

h3.fw-normal {
	font-weight: 400;
	color: var(--evcc-default-text);
}

h3.fw-normal small {
	color: var(--evcc-gray);
	font-weight: 300;
}

h4.fw-normal {
	font-weight: 500;
	color: var(--evcc-default-text);
	font-size: 1.25rem;
}

@media (max-width: 576px) {
	.header-outer .sticky-top {
		top: 4.5rem;
	}
	
	.card-body {
		padding: 1rem;
	}
	
	.card-text {
		font-size: 1.25rem;
	}
}
</style>