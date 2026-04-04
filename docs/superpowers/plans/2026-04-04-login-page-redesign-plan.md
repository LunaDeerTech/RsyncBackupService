# Login Page Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework the login page into a single-card, product-style entry page without changing authentication behavior.

**Architecture:** Keep the existing login route, session handling, and UI component stack intact. Limit the redesign to the `LoginView` page and its page-level test so the work stays visual and behavioral rather than architectural.

**Tech Stack:** Vue 3, TypeScript, Pinia, Vue Router, Vitest, Testing Library, existing UI token system and shared components.

---

### Task 1: Lock The New Login Expectations In Tests

**Files:**
- Modify: `web/src/views/LoginView.spec.ts`
- Test: `web/src/views/LoginView.spec.ts`

- [ ] **Step 1: Write the failing test for removing demo defaults**

```ts
it("renders a product-style login page without demo defaults", async () => {
  const router = createRouter()
  await router.push("/login")
  await router.isReady()

  render(LoginView, {
    global: {
      plugins: [router],
    },
  })

  expect(screen.getByText("Rsync Backup Service")).toBeInTheDocument()
  expect(screen.getByLabelText("用户名")).toHaveValue("")
  expect(screen.queryByLabelText("能力概览")).not.toBeInTheDocument()
  expect(screen.getByRole("button", { name: /主题/ })).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `npm --prefix web run test -- src/views/LoginView.spec.ts`
Expected: FAIL because the current page still pre-fills `admin` and still renders the demo-style highlights area.

- [ ] **Step 3: Keep the existing submit-flow test as the login behavior guardrail**

```ts
it("submits login form and stores returned token pair", async () => {
  await fireEvent.update(screen.getByLabelText("用户名"), "admin")
  await fireEvent.update(screen.getByLabelText("密码"), "secret")
  await fireEvent.click(screen.getByRole("button", { name: "登录" }))

  await waitFor(() => {
    expect(login).toHaveBeenCalledWith({
      username: "admin",
      password: "secret",
    })
  })
})
```

- [ ] **Step 4: Re-run the focused test after the spec assertions are in place**

Run: `npm --prefix web run test -- src/views/LoginView.spec.ts`
Expected: still FAIL until the page layout is updated.

### Task 2: Implement The Single-Card Product Login Page

**Files:**
- Modify: `web/src/views/LoginView.vue`
- Test: `web/src/views/LoginView.spec.ts`

- [ ] **Step 1: Reset the form defaults to product behavior**

```ts
const form = reactive({
  username: "",
  password: "",
})
```

- [ ] **Step 2: Replace the split hero layout with a centered single-card structure**

```vue
<div class="login-view" data-testid="login-view">
  <div class="login-view__container">
    <header class="login-view__brand">
      <p class="login-view__product">Rsync Backup Service</p>
      <p class="login-view__intro">使用账户访问备份控制台。</p>
    </header>

    <AppCard class="login-view__card">
      <!-- compact header, secondary theme switch, form, error, hint -->
    </AppCard>
  </div>
</div>
```

- [ ] **Step 3: Rewrite the page styles around a restrained background and centered card**

```css
.login-view {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: var(--space-6);
  background:
    radial-gradient(circle at top, color-mix(in srgb, var(--primary-500) 10%, transparent), transparent 42%),
    var(--surface-base);
}

.login-view__container {
  width: min(100%, 28rem);
  display: grid;
  gap: var(--space-5);
}
```

- [ ] **Step 4: Run the focused page test to verify the redesign passes behavior and structure checks**

Run: `npm --prefix web run test -- src/views/LoginView.spec.ts`
Expected: PASS.

### Task 3: Validate Build And Manual Preview

**Files:**
- Modify: none unless verification exposes a page-level issue
- Test: `web/src/views/LoginView.spec.ts`

- [ ] **Step 1: Build the frontend bundle**

Run: `npm --prefix web run build`
Expected: PASS.

- [ ] **Step 2: Start or reuse the local preview server and open the login route**

Run: `npm --prefix web run dev -- --host 0.0.0.0`
Expected: Vite starts successfully and the login page can be opened in a browser.

- [ ] **Step 3: Ask the user to review the visual result**

Prompt the user to inspect the browser preview and confirm whether the redesigned login page now reads as a product login entry.
