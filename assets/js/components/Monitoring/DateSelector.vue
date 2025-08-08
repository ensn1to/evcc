<template>
	<div class="d-flex justify-content-end align-items-center gap-2">
		<DateNavigatorButton
			prev
			:disabled="!hasPrevDay"
			:onClick="emitPrevDay"
			data-testid="navigate-prev-day"
		/>
		<CustomSelect
			id="monitoringDate"
			:options="dateOptions"
			:selected="selectedDate"
			@change="emitDate($event.target.value)"
		>
			<button
				class="btn btn-sm border-0 h-100 date-button"
				data-testid="navigate-date"
			>
				{{ formattedDate }}
			</button>
		</CustomSelect>
		<DateNavigatorButton
			next
			:disabled="!hasNextDay"
			:onClick="emitNextDay"
			data-testid="navigate-next-day"
		/>
	</div>
</template>

<script>
import CustomSelect from "../Helper/CustomSelect.vue";
import DateNavigatorButton from "../Sessions/DateNavigatorButton.vue";
import formatter from "../../mixins/formatter";

export default {
	name: "DateSelector",
	components: {
		CustomSelect,
		DateNavigatorButton,
	},
	mixins: [formatter],
	props: {
		selectedDate: { type: String, required: true },
	},
	emits: ["update-date"],
	computed: {
		currentDate() {
			return new Date(this.selectedDate);
		},
		formattedDate() {
			return this.currentDate.toLocaleDateString();
		},
		hasPrevDay() {
			// Allow going back up to 30 days
			const thirtyDaysAgo = new Date();
			thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);
			return this.currentDate > thirtyDaysAgo;
		},
		hasNextDay() {
			// Don't allow future dates
			const today = new Date();
			today.setHours(23, 59, 59, 999);
			return this.currentDate < today;
		},
		dateOptions() {
			const options = [];
			const today = new Date();
			
			// Generate last 30 days
			for (let i = 0; i < 30; i++) {
				const date = new Date(today);
				date.setDate(today.getDate() - i);
				const dateStr = date.toISOString().split('T')[0];
				options.push({
					value: dateStr,
					name: date.toLocaleDateString(),
				});
			}
			
			return options;
		},
	},
	methods: {
		emitPrevDay() {
			if (!this.hasPrevDay) return;
			const prevDate = new Date(this.currentDate);
			prevDate.setDate(prevDate.getDate() - 1);
			this.$emit("update-date", prevDate.toISOString().split('T')[0]);
		},
		emitNextDay() {
			if (!this.hasNextDay) return;
			const nextDate = new Date(this.currentDate);
			nextDate.setDate(nextDate.getDate() + 1);
			this.$emit("update-date", nextDate.toISOString().split('T')[0]);
		},
		emitDate(dateStr) {
			this.$emit("update-date", dateStr);
		},
	},
};
</script>

<style scoped>
.date-button {
	min-width: 120px;
	padding: 0.5rem 1rem;
	border: 1px solid var(--evcc-box-border) !important;
	border-radius: 8px;
	background-color: var(--evcc-background);
	color: var(--evcc-default-text);
	font-size: 0.875rem;
	transition: all 0.15s ease-in-out;
}

.date-button:hover {
	border-color: var(--evcc-primary) !important;
	background-color: var(--evcc-gray-light);
}

.date-button:focus {
	border-color: var(--evcc-primary) !important;
	box-shadow: 0 0 0 0.2rem rgba(var(--evcc-primary-rgb), 0.25);
	outline: 0;
}
</style>