import { expect, test } from "@playwright/test"

test.beforeEach(({ page }) => {
  page.on("pageerror", (error) => {
    throw error
  })
})

test("explores the fixture and opens the selected package inspector", async ({ page }) => {
  await page.goto("/")
  await expect(page.getByRole("heading", { name: "personal-portfolio" })).toBeVisible()
  await expect(page.getByLabel("Interactive dependency graph")).toBeVisible()
  await expect(page.getByLabel("Queue counts").getByText("185", { exact: true })).toBeVisible()

  await page.getByRole("searchbox").fill("argparse")
  await page.getByRole("group", { name: /argparse version 2.0.1/i }).click()
  const inspector = page.getByLabel("Selected package inspector", { exact: true })
  await expect(inspector.getByRole("heading", { name: /argparse/ })).toBeVisible()
  await expect(inspector.getByText("Policy violation", { exact: true })).toBeVisible()
})

test("filters packages and restores the graph", async ({ page }) => {
  await page.goto("/")
  await page.getByRole("searchbox").fill("does-not-exist")
  const canvas = page.getByRole("region", { name: "Dependency graph canvas" })
  await expect(canvas.getByRole("status").getByText("No packages match these filters.", { exact: true })).toBeVisible()
  await page.getByRole("searchbox").fill("")
  await expect(canvas.getByRole("status")).toHaveCount(0)
  await expect(page.getByLabel("Interactive dependency graph")).toBeVisible()
})

test("reset layout recovers a graph saved outside the viewport", async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem("dependency-audit-view:v4:classic-demo", JSON.stringify({
      selectedNodeId: null,
      filters: { search: "", violationsOnly: false, completeGraph: false },
      pinnedPositions: {},
      collapsedNodeIds: [],
      annotations: {},
      canvasBoxes: [],
      canvasArrows: [],
      viewport: { x: 50000, y: 50000, zoom: 1 },
      summaryOpen: false,
      inspectorOpen: false,
      activityOpen: false,
    }))
  })
  await page.goto("/")
  await page.getByRole("button", { name: "Reset graph layout" }).click()
  await expect(page.getByRole("group", { name: /eslint version 9.39.5/i })).toBeVisible()
})

test("switches from the overview to the entire dependency graph", async ({ page }, testInfo) => {
  await page.goto("/")
  if (testInfo.project.name === "mobile") {
    await page.getByRole("button", { name: "Open audit summary and filters" }).click()
  }
  await page.getByRole("checkbox", { name: "Entire dependency graph" }).check()
  await expect(page.getByRole("group", { name: /lightningcss version 1.32.0/i })).toBeAttached()
})

test("creates, connects, persists, and deletes presentation boxes", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop", "Pointer-connection workflow is covered on desktop")
  await page.goto("/")
  const graph = page.getByLabel("Interactive dependency graph")

  await page.getByRole("button", { name: "Add presentation box" }).click()
  await expect(page.getByText(/Click empty canvas to place a box/)).toBeVisible()
  await graph.click({ position: { x: 310, y: 260 } })
  await page.getByRole("textbox", { name: "Presentation box label" }).fill("Review path")
  await page.getByRole("textbox", { name: "Presentation box label" }).press("Enter")

  await page.getByRole("button", { name: "Add presentation box" }).click()
  await graph.click({ position: { x: 650, y: 360 } })
  await page.getByRole("textbox", { name: "Presentation box label" }).fill("Decision")
  await page.getByRole("textbox", { name: "Presentation box label" }).press("Enter")

  const firstNode = page.locator(".react-flow__node-canvasBox").filter({ hasText: "Review path" })
  const secondNode = page.locator(".react-flow__node-canvasBox").filter({ hasText: "Decision" })
  const source = firstNode.locator(".canvas-box__handle--source")
  const target = secondNode.locator(".canvas-box__handle--target")
  await source.dragTo(target)
  await expect(page.locator(".react-flow__edge.canvas-arrow")).toHaveCount(1)

  await expect.poll(() => page.evaluate(() => {
    const raw = window.localStorage.getItem("dependency-audit-view:v4:classic-demo")
    if (!raw) return 0
    return (JSON.parse(raw).canvasBoxes ?? []).length
  })).toBe(2)
  await page.reload()
  await expect(page.locator(".react-flow__node-canvasBox").filter({ hasText: "Review path" })).toBeVisible()
  await expect(page.locator(".react-flow__node-canvasBox").filter({ hasText: "Decision" })).toBeVisible()
  await expect(page.locator(".react-flow__edge.canvas-arrow")).toHaveCount(1)

  await page.locator(".react-flow__node-canvasBox").filter({ hasText: "Review path" }).click()
  await page.keyboard.press("Delete")
  await expect(page.locator(".react-flow__node-canvasBox").filter({ hasText: "Review path" })).toHaveCount(0)
  await expect(page.locator(".react-flow__edge.canvas-arrow")).toHaveCount(0)

  const auditNode = page.getByRole("group", { name: /eslint version 9.39.5/i })
  await auditNode.click()
  await page.keyboard.press("Delete")
  await expect(auditNode).toBeVisible()
})

test("undo and redo restore a committed box label", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop", "Keyboard history workflow is covered on desktop")
  await page.goto("/")
  const graph = page.getByLabel("Interactive dependency graph")
  await page.getByRole("button", { name: "Add presentation box" }).click()
  await graph.click({ position: { x: 400, y: 300 } })
  await page.getByRole("textbox", { name: "Presentation box label" }).fill("Release note")
  await page.getByRole("textbox", { name: "Presentation box label" }).press("Enter")

  await page.getByRole("button", { name: "Undo presentation change" }).click()
  await expect(page.locator(".react-flow__node-canvasBox").filter({ hasText: "Untitled" })).toBeVisible()
  await page.getByRole("button", { name: "Redo presentation change" }).click()
  await expect(page.locator(".react-flow__node-canvasBox").filter({ hasText: "Release note" })).toBeVisible()

  await page.getByRole("button", { name: "Add presentation box" }).click()
  await expect(page.getByText(/Click empty canvas to place a box/)).toBeVisible()
  await page.keyboard.press("Escape")
  await expect(page.getByText(/Click empty canvas to place a box/)).toHaveCount(0)
  await expect(page.locator(".react-flow__node-canvasBox")).toHaveCount(1)
})

test("opens and dismisses the mobile summary sheet", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile", "Mobile-only interaction")
  await page.goto("/")
  await page.getByRole("button", { name: "Open audit summary and filters" }).click()
  await expect(page.getByRole("complementary", { name: "Audit summary and filters" })).toBeVisible()
  await page.getByRole("button", { name: "Close audit summary" }).click()
  await expect(page.getByRole("button", { name: "Open audit summary and filters" })).toBeFocused()
})
