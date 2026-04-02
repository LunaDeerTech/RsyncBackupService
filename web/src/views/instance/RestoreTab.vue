<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue"

import { verifyPassword } from "../../api/auth"
import { listRestoreRecords, listSnapshots, startRestore } from "../../api/backups"
import type { BackupRecord, InstanceDetail, RestoreRecord } from "../../api/types"
import { ApiError } from "../../api/client"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppDialog from "../../components/ui/AppDialog.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppPasswordInput from "../../components/ui/AppPasswordInput.vue"
import AppSelect from "../../components/ui/AppSelect.vue"
import AppSwitch from "../../components/ui/AppSwitch.vue"
import AppTable from "../../components/ui/AppTable.vue"
import AppTag from "../../components/ui/AppTag.vue"
import { formatBackupType, formatDateTime, formatStatusLabel, statusTone } from "../../utils/formatters"

const props = withDefaults(
	defineProps<{
		instanceId: number
		instance: InstanceDetail
		relayMode?: boolean
		relayModeHint?: string
		relayModeTitle?: string
	}>(),
	{
		relayMode: false,
		relayModeHint: "",
		relayModeTitle: "中继模式",
	},
)

const snapshots = ref<BackupRecord[]>([])
const restoreRecords = ref<RestoreRecord[]>([])
const errorMessage = ref("")
const successMessage = ref("")
const confirmOpen = ref(false)
const password = ref("")
const verifyError = ref("")
const isSubmitting = ref(false)

const form = reactive({
	backupRecordId: "",
	restoreTargetPath: props.instance.source_path,
	overwrite: true,
})

function formatStorageTargetContext(storageTargetId: number): string {
	return `存储目标 #${storageTargetId}`
}

const snapshotOptions = computed(() =>
	snapshots.value.map((snapshot) => ({
		value: String(snapshot.id),
		label: `${formatDateTime(snapshot.started_at)} · ${formatBackupType(snapshot.backup_type)} · ${formatStorageTargetContext(snapshot.storage_target_id)}`,
	})),
)

const selectedSnapshot = computed(() =>
	snapshots.value.find((snapshot) => snapshot.id === Number.parseInt(form.backupRecordId, 10)),
)

const riskMessage = computed(() => {
	const snapshotLabel = selectedSnapshot.value
		? `${formatDateTime(selectedSnapshot.value.started_at)}（${formatStorageTargetContext(selectedSnapshot.value.storage_target_id)}）`
		: "所选快照"
	const targetPath = form.restoreTargetPath.trim() || props.instance.source_path

	if (form.overwrite) {
		return `即将用 ${snapshotLabel} 的快照覆盖 ${targetPath}，当前文件将被替换，此操作不可撤销。`
	}

	return `即将恢复 ${snapshotLabel} 的快照到 ${targetPath}，请确认路径有足够空间并避免与在线业务目录混淆。`
})

async function loadData(): Promise<void> {
	errorMessage.value = ""

	try {
		const snapshotItems = await listSnapshots(props.instanceId)
		snapshots.value = snapshotItems

		if (form.backupRecordId === "" && snapshotItems.length > 0) {
			form.backupRecordId = String(snapshotItems[0].id)
		}

		restoreRecords.value = await listRestoreRecords(props.instanceId)
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载恢复数据失败。"
	}
}

function openConfirm(): void {
	if (form.backupRecordId === "" && snapshots.value.length > 0) {
		form.backupRecordId = String(snapshots.value[0].id)
	}

	if (form.backupRecordId === "" || form.restoreTargetPath.trim() === "") {
		return
	}

	verifyError.value = ""
	password.value = ""
	confirmOpen.value = true
}

function closeConfirm(): void {
	if (isSubmitting.value) {
		return
	}

	confirmOpen.value = false
	verifyError.value = ""
	password.value = ""
}

async function submitRestore(): Promise<void> {
	verifyError.value = ""
	isSubmitting.value = true

	try {
		const verification = await verifyPassword(password.value)
		const createdRecord = await startRestore({
			instance_id: props.instanceId,
			backup_record_id: Number.parseInt(form.backupRecordId, 10),
			restore_target_path: form.restoreTargetPath.trim(),
			overwrite: form.overwrite,
			verify_token: verification.verify_token,
		})

		successMessage.value = `恢复任务已提交，记录 ID ${createdRecord.id}。`
		confirmOpen.value = false
		password.value = ""
		await loadData()
	} catch (error) {
		verifyError.value = error instanceof ApiError ? error.message : "恢复提交失败。"
	} finally {
		isSubmitting.value = false
	}
}

onMounted(() => {
	void loadData()
})

watch(
	() => props.instance.source_path,
	(nextValue) => {
		if (form.restoreTargetPath.trim() === "") {
			form.restoreTargetPath = nextValue
		}
	},
)
</script>

<template>
	<section class="page-view">
		<AppNotification
			v-if="props.relayMode"
			:title="props.relayModeTitle"
			tone="warning"
			:description="props.relayModeHint || '所选实例存在远程到远程链路。恢复执行时会通过本机缓存目录中继，请确认剩余空间充足。'"
		/>
		<AppNotification v-if="errorMessage" title="恢复页面加载失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="恢复任务已创建" tone="success" :description="successMessage" />

		<section class="page-two-column">
			<AppCard title="恢复参数" description="选择可恢复快照，指定目标路径并确认覆盖语义。">
				<form class="page-stack" @submit.prevent="openConfirm">
					<AppFormField label="快照 / 归档" required>
						<AppSelect v-model="form.backupRecordId" :options="snapshotOptions" />
					</AppFormField>

					<AppFormField label="恢复目标路径" required>
						<AppInput v-model="form.restoreTargetPath" />
					</AppFormField>

					<AppFormField label="覆盖原位置">
						<AppSwitch v-model="form.overwrite" />
					</AppFormField>

					<AppNotification
						title="风险提示"
						tone="danger"
						:description="riskMessage"
					/>

					<div v-if="selectedSnapshot" class="page-stack">
						<p class="page-section__title">所选恢复源</p>
						<div class="page-action-row--wrap">
							<AppTag :tone="statusTone(selectedSnapshot.status)">{{ formatStatusLabel(selectedSnapshot.status) }}</AppTag>
							<AppTag tone="info">{{ formatStorageTargetContext(selectedSnapshot.storage_target_id) }}</AppTag>
							<span class="page-muted">{{ formatDateTime(selectedSnapshot.started_at) }}</span>
						</div>
						<p class="page-mono">{{ selectedSnapshot.snapshot_path }}</p>
					</div>

					<AppButton
						type="submit"
						:disabled="snapshotOptions.length === 0 || form.restoreTargetPath.trim() === ''"
						@click="openConfirm"
					>
						开始恢复
					</AppButton>
				</form>
			</AppCard>

			<AppCard title="恢复记录" description="显示当前实例的恢复任务记录。">
				<AppTable
					:rows="restoreRecords"
					:columns="[
						{ key: 'backup_record_id', label: '恢复源' },
						{ key: 'restore_target_path', label: '目标路径' },
						{ key: 'overwrite', label: '覆盖' },
						{ key: 'status', label: '状态' },
						{ key: 'started_at', label: '开始时间' },
					]"
					row-key="id"
				>
					<template #cell-overwrite="{ value }">
						<AppTag :tone="value ? 'danger' : 'info'">{{ value ? "覆盖" : "新位置" }}</AppTag>
					</template>
					<template #cell-status="{ value }">
						<AppTag :tone="statusTone(String(value))">{{ formatStatusLabel(String(value)) }}</AppTag>
					</template>
					<template #cell-started_at="{ row }">
						<div class="page-stack">
							<span>{{ formatDateTime(row.started_at) }}</span>
							<span class="page-muted">完成 {{ formatDateTime(row.finished_at) }}</span>
						</div>
					</template>
				</AppTable>
				<AppEmpty v-if="restoreRecords.length === 0" title="尚无恢复记录" compact />
			</AppCard>
		</section>

		<AppDialog :open="confirmOpen" title="确认恢复" tone="danger" @close="closeConfirm">
			<div class="page-stack">
				<p class="page-danger-copy">{{ riskMessage }}</p>
				<p class="page-danger-copy">二次认证：请输入当前账户密码以换取一次性 verify token。覆盖模式会直接替换目标路径内容。</p>
				<AppFormField label="确认密码" required>
					<AppPasswordInput v-model="password" autocomplete="current-password" />
				</AppFormField>
				<AppNotification v-if="verifyError" title="恢复未提交" tone="danger" :description="verifyError" />
			</div>

			<template #actions>
				<AppButton variant="ghost" @click="closeConfirm">取消</AppButton>
				<AppButton variant="danger" :loading="isSubmitting" @click="submitRestore">确认恢复</AppButton>
			</template>
		</AppDialog>
	</section>
</template>