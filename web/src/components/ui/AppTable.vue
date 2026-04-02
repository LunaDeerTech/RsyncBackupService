<script setup lang="ts">
import { useAttrs } from "vue"

export interface TableColumn<T extends Record<string, unknown> = Record<string, unknown>> {
	key: keyof T | string
	label: string
}

export interface AppTableProps<T extends Record<string, unknown> = Record<string, unknown>> {
	rows: T[]
	columns: TableColumn<T>[]
	dense?: boolean
	rowKey?: keyof T | ((row: T, rowIndex: number) => string | number)
}

type TableRow = Record<string, unknown>

defineOptions({
	inheritAttrs: false,
})

const props = withDefaults(defineProps<AppTableProps<TableRow>>(), {
	dense: false,
	rowKey: undefined,
})

const attrs = useAttrs()

function resolveCellValue(row: TableRow, key: string): unknown {
	return row[key]
}

function formatCellValue(value: unknown): string {
	if (value === null || value === undefined || value === "") {
		return "—"
	}

	if (typeof value === "boolean") {
		return value ? "是" : "否"
	}

	if (typeof value === "number" || typeof value === "bigint") {
		return String(value)
	}

	if (Array.isArray(value)) {
		return value.join(", ")
	}

	return String(value)
	}

function slotName(key: string): string {
	return `cell-${key}`
}

function resolveRowKey(row: TableRow, rowIndex: number): string | number {
	if (typeof props.rowKey === "function") {
		return props.rowKey(row, rowIndex)
	}

	if (props.rowKey) {
		const value = row[String(props.rowKey)]

		if (typeof value === "string" || typeof value === "number") {
			return value
		}
	}

	if (typeof row.id === "string" || typeof row.id === "number") {
		return row.id
	}

	return rowIndex
}
</script>

<template>
	<div class="app-table-shell" :data-density="dense ? 'dense' : 'default'">
		<table v-bind="attrs" class="app-table" :data-density="dense ? 'dense' : 'default'">
			<thead>
				<tr>
					<th v-for="column in columns" :key="String(column.key)" scope="col">
						{{ column.label }}
					</th>
				</tr>
			</thead>
			<tbody>
				<tr v-if="rows.length === 0">
					<td class="app-table__empty" :colspan="columns.length">暂无数据</td>
				</tr>
				<tr v-for="(row, rowIndex) in rows" v-else :key="resolveRowKey(row, rowIndex)">
					<td v-for="column in columns" :key="String(column.key)" :data-column-key="String(column.key)">
						<slot
							:name="slotName(String(column.key))"
							:row="row"
							:value="resolveCellValue(row, String(column.key))"
						>
							<span class="app-table__cell-value">
								{{ formatCellValue(resolveCellValue(row, String(column.key))) }}
							</span>
						</slot>
					</td>
				</tr>
			</tbody>
		</table>
	</div>
</template>

<style scoped>
.app-table-shell {
	overflow-x: auto;
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 96%, transparent);
	border-radius: var(--radius-card);
	background: color-mix(in srgb, var(--surface-panel-solid) 98%, transparent);
	box-shadow: inset 0 1px 0 color-mix(in srgb, white 24%, transparent);
	backdrop-filter: none;
	-webkit-backdrop-filter: none;
}

.app-table {
	width: 100%;
	border-collapse: separate;
	border-spacing: 0;
	font-size: 0.92rem;
	font-variant-numeric: tabular-nums;
	backdrop-filter: none;
	-webkit-backdrop-filter: none;
}

.app-table[data-density="dense"] {
	font-size: 0.85rem;
}

.app-table th,
.app-table td {
	padding: 0.9rem 1rem;
	text-align: left;
	border-bottom: var(--border-width) solid color-mix(in srgb, var(--border-default) 82%, transparent);
	backdrop-filter: none;
	-webkit-backdrop-filter: none;
}

.app-table[data-density="dense"] th,
.app-table[data-density="dense"] td {
	padding: 0.7rem 0.9rem;
	}

.app-table th {
	position: sticky;
	top: 0;
	background: color-mix(in srgb, var(--surface-elevated) 96%, var(--surface-panel-solid));
	color: var(--text-muted);
	font-size: 0.76rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	white-space: nowrap;
	z-index: 1;
}

.app-table tbody tr {
	transition:
		background-color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease;
}

.app-table tbody tr:hover {
	background: color-mix(in srgb, var(--surface-accent-soft) 64%, var(--surface-panel-solid));
	box-shadow: inset 2px 0 0 color-mix(in srgb, var(--primary-500) 22%, transparent);
}

.app-table tbody tr:last-child td {
	border-bottom: none;
}

.app-table__cell-value {
	display: inline-flex;
	align-items: center;
	min-height: 1.25rem;
	color: var(--text-strong);
	line-height: 1.4;
}

.app-table__empty {
	padding: 1.3rem 1rem;
	color: var(--text-muted);
	text-align: center;
	font-size: 0.9rem;
	}

@media (max-width: 720px) {
	.app-table th,
	.app-table td {
		padding-inline: 0.82rem;
	}
}
</style>