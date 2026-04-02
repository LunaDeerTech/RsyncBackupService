import { fireEvent, render, screen } from "@testing-library/vue"
import { defineComponent, ref } from "vue"

import AppButton from "./AppButton.vue"
import AppFormField from "./AppFormField.vue"
import AppInput from "./AppInput.vue"
import AppPasswordInput from "./AppPasswordInput.vue"
import AppSelect from "./AppSelect.vue"

describe("AppButton", () => {
	it("renders danger button with error semantics instead of brand color", () => {
		render(AppButton, {
			props: {
				variant: "danger",
			},
			slots: {
				default: "删除实例",
			},
		})

		expect(screen.getByRole("button", { name: "删除实例" })).toHaveAttribute("data-variant", "danger")
	})

	it("disables interaction while loading", () => {
		render(AppButton, {
			props: {
				loading: true,
			},
			slots: {
				default: "保存策略",
			},
		})

		const button = screen.getByRole("button", { name: "保存策略" })

		expect(button).toBeDisabled()
		expect(button).toHaveAttribute("aria-busy", "true")
	})
})

describe("AppInput and AppFormField", () => {
	it("renders an invalid field with a clear error message", () => {
		const FieldHarness = defineComponent({
			components: {
				AppFormField,
				AppInput,
			},
			setup() {
				const value = ref("")
				return {
					value,
				}
			},
			template: `
				<AppFormField label="实例名称" error="请输入实例名称" required>
					<AppInput v-model="value" invalid placeholder="例如 prod-app" />
				</AppFormField>
			`,
		})

		const { container } = render(FieldHarness)
		const input = screen.getByLabelText("实例名称")

		expect(input).toHaveAttribute("aria-invalid", "true")
		expect(screen.getByRole("alert")).toHaveTextContent("请输入实例名称")
		expect(container.querySelector(".app-form-field[data-invalid='true']")).not.toBeNull()
	})

	it("keeps disabled inputs out of interaction", () => {
		render(AppInput, {
			props: {
				modelValue: "/srv/source",
				disabled: true,
			},
			attrs: {
				"aria-label": "源路径",
			},
		})

		expect(screen.getByLabelText("源路径")).toBeDisabled()
	})

	it("keeps label targeting intact when the consumer passes an explicit control id", () => {
		const FieldHarness = defineComponent({
			components: {
				AppFormField,
				AppInput,
			},
			setup() {
				const value = ref("prod-main")
				return {
					value,
				}
			},
			template: `
				<AppFormField label="实例名称">
					<AppInput id="instance-name-input" v-model="value" />
				</AppFormField>
			`,
		})

		render(FieldHarness)

		expect(screen.getByLabelText("实例名称")).toHaveAttribute("id", "instance-name-input")
	})
})

describe("AppPasswordInput", () => {
	it("toggles the input type from password to text", async () => {
		const PasswordHarness = defineComponent({
			components: {
				AppPasswordInput,
			},
			setup() {
				const value = ref("secret-value")
				return {
					value,
				}
			},
			template: `<AppPasswordInput v-model="value" aria-label="数据库密码" />`,
		})

		render(PasswordHarness)

		const input = screen.getByLabelText("数据库密码")

		expect(input).toHaveAttribute("type", "password")

		await fireEvent.click(screen.getByRole("button", { name: "显示密码" }))

		expect(input).toHaveAttribute("type", "text")
	})
})

describe("AppSelect", () => {
	it("opens a themed listbox and supports keyboard selection", async () => {
		const SelectHarness = defineComponent({
			components: {
				AppSelect,
			},
			setup() {
				const value = ref("local")
				return {
					value,
					options: [
						{ value: "local", label: "本地目录" },
						{ value: "ssh", label: "SSH 远端" },
					],
				}
			},
			template: `
				<div>
					<AppSelect v-model="value" :options="options" aria-label="存储目标类型" />
					<p data-testid="selected-value">{{ value }}</p>
				</div>
			`,
		})

		render(SelectHarness)

		const trigger = screen.getByRole("combobox", { name: "存储目标类型" })

		expect(screen.queryByRole("listbox")).not.toBeInTheDocument()

		await fireEvent.click(trigger)
		expect(screen.getByRole("listbox")).toBeInTheDocument()

		await fireEvent.keyDown(trigger, { key: "ArrowDown" })
		await fireEvent.keyDown(trigger, { key: "Enter" })

		expect(screen.getByTestId("selected-value")).toHaveTextContent("ssh")
		expect(screen.queryByRole("listbox")).not.toBeInTheDocument()
	})

	it("keeps placeholder text until a value is selected and closes on tab", async () => {
		const SelectHarness = defineComponent({
			components: {
				AppSelect,
			},
			setup() {
				const value = ref("")
				return {
					value,
					options: [
						{ value: "local", label: "本地目录" },
						{ value: "ssh", label: "SSH 远端" },
					],
				}
			},
			template: `<AppSelect v-model="value" :options="options" placeholder="请选择存储目标" aria-label="空选择器" />`,
		})

		render(SelectHarness)

		const trigger = screen.getByRole("combobox", { name: "空选择器" })

		expect(trigger).toHaveTextContent("请选择存储目标")

		await fireEvent.click(trigger)
		expect(screen.getByRole("listbox")).toBeInTheDocument()

		await fireEvent.keyDown(trigger, { key: "Tab" })

		expect(screen.queryByRole("listbox")).not.toBeInTheDocument()
		expect(trigger).toHaveTextContent("请选择存储目标")
	})

	it("keeps focus on the trigger after mouse selection", async () => {
		const SelectHarness = defineComponent({
			components: {
				AppSelect,
			},
			setup() {
				const value = ref("local")
				return {
					value,
					options: [
						{ value: "local", label: "本地目录" },
						{ value: "ssh", label: "SSH 远端" },
					],
				}
			},
			template: `<AppSelect v-model="value" :options="options" aria-label="鼠标选择器" />`,
		})

		render(SelectHarness)

		const trigger = screen.getByRole("combobox", { name: "鼠标选择器" })

		await fireEvent.click(trigger)
		const option = screen.getByRole("option", { name: "SSH 远端" })

		await fireEvent.mouseDown(option)
		await fireEvent.click(option)

		expect(trigger).toHaveFocus()
		expect(screen.queryByRole("listbox")).not.toBeInTheDocument()
	})
})