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

  await page.getByRole("group", { name: /legacy-widget version 1.4.0/i }).click()
  const inspector = page.getByLabel("Selected package inspector", { exact: true })
  await expect(inspector.getByRole("heading", { name: /legacy-widget/ })).toBeVisible()
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
    window.localStorage.setItem("dependency-audit-view:v2:classic-demo", JSON.stringify({
      selectedNodeId: null,
      filters: { search: "", violationsOnly: false, directOnly: false },
      pinnedPositions: {},
      collapsedNodeIds: [],
      annotations: {},
      viewport: { x: 50000, y: 50000, zoom: 1 },
      summaryOpen: false,
      inspectorOpen: false,
      activityOpen: false,
    }))
  })
  await page.goto("/")
  await page.getByRole("button", { name: "Reset graph layout" }).click()
  await expect(page.getByRole("group", { name: /legacy-widget version 1.4.0/i })).toBeVisible()
})

test("opens and dismisses the mobile summary sheet", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile", "Mobile-only interaction")
  await page.goto("/")
  await page.getByRole("button", { name: "Open audit summary and filters" }).click()
  await expect(page.getByRole("complementary", { name: "Audit summary and filters" })).toBeVisible()
  await page.getByRole("button", { name: "Close audit summary" }).click()
  await expect(page.getByRole("button", { name: "Open audit summary and filters" })).toBeFocused()
})
