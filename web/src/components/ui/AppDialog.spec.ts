import { fireEvent, render, screen, waitFor } from "@testing-library/vue"
import { defineComponent, ref } from "vue"

import AppDialog from "./AppDialog.vue"
import AppTabs from "./AppTabs.vue"

describe("AppDialog", () => {
	it("keeps destructive dialog copy visible and focus-trapped", async () => {
		render(AppDialog, {
			props: {
				open: true,
				title: "删除备份实例",
				tone: "danger",
			},
			slots: {
				default: "<p>此操作不可撤销，相关策略也会一并移除。</p>",
				actions: '<button type="button">取消</button><button type="button">确认删除</button>',
			},
		})

		const dialog = screen.getByRole("dialog", { name: "删除备份实例" })
		const cancelButton = screen.getByRole("button", { name: "取消" })
		const confirmButton = screen.getByRole("button", { name: "确认删除" })

		expect(dialog).toBeInTheDocument()
		expect(dialog).toHaveAttribute("data-tone", "danger")
		expect(screen.getByText("此操作不可撤销，相关策略也会一并移除。")).toBeInTheDocument()

		await waitFor(() => {
			expect(cancelButton).toHaveFocus()
		})

		await fireEvent.keyDown(cancelButton, { key: "Tab" })
		expect(confirmButton).toHaveFocus()

		await fireEvent.keyDown(confirmButton, { key: "Tab" })
		expect(cancelButton).toHaveFocus()

		await fireEvent.keyDown(cancelButton, { key: "Tab", shiftKey: true })
		expect(confirmButton).toHaveFocus()
	})

	it("emits close when escape is pressed", async () => {
		const { emitted } = render(AppDialog, {
			props: {
				open: true,
				title: "退出确认",
			},
			slots: {
				actions: '<button type="button">关闭</button>',
			},
		})

		await fireEvent.keyDown(screen.getByRole("dialog", { name: "退出确认" }), { key: "Escape" })

		expect(emitted().close).toHaveLength(1)
	})
})

describe("AppTabs", () => {
	it("supports arrow-key navigation between tabs", async () => {
		const TabsHarness = defineComponent({
			components: {
				AppTabs,
			},
			setup() {
				const activeTab = ref("general")
				return {
					activeTab,
					tabs: [
						{ value: "general", label: "常规设置" },
						{ value: "danger", label: "危险操作" },
					],
				}
			},
			template: `
				<AppTabs v-model="activeTab" :tabs="tabs">
					<template #default="{ activeTab: currentTab }">
						<div>{{ currentTab?.label }}</div>
					</template>
				</AppTabs>
			`,
		})

		render(TabsHarness)

		const generalTab = screen.getByRole("tab", { name: "常规设置" })
		const dangerTab = screen.getByRole("tab", { name: "危险操作" })
		const initialPanel = screen.getByRole("tabpanel")
		const initialPanelId = initialPanel.getAttribute("id")
		const dangerPanelId = dangerTab.getAttribute("aria-controls")

		generalTab.focus()
		await fireEvent.keyDown(generalTab, { key: "ArrowRight" })

		expect(dangerTab).toHaveAttribute("aria-selected", "true")
		expect(dangerTab).toHaveFocus()
		expect(dangerTab).toHaveAttribute("aria-controls", dangerPanelId)
		expect(screen.getByRole("tabpanel")).toHaveAttribute("id", dangerPanelId)
		expect(generalTab).toHaveAttribute("aria-controls", initialPanelId)
		expect(screen.getByRole("tabpanel")).toHaveAttribute("aria-labelledby", dangerTab.getAttribute("id"))
	})
})