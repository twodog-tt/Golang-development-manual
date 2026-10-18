"""Exercise the public production site in Chromium after a deployment."""
import argparse
import json
from pathlib import Path
import re
from playwright.sync_api import sync_playwright, expect

ARTICLE = "/topics/18-web3-payments-stablecoin/S-PAY-07-postgresql-double-entry-ledger-lab/"
OUTBOX = "/topics/middleware/distributed/S-DIST-06-outbox-inbox-fault-lab/"


def check(base, release, output):
    output.mkdir(parents=True, exist_ok=True)
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch()
        context = browser.new_context(base_url=base, viewport={"width": 1440, "height": 1000})
        context.set_default_timeout(20000)
        context.set_default_navigation_timeout(60000)
        context.tracing.start(screenshots=True, snapshots=True, sources=True)
        page = context.new_page()
        page.on("pageerror", lambda error: errors.append(str(error)))
        try:
            response = page.goto("/", wait_until="networkidle")
            assert response.status == 200, "Homepage HTTP status"
            expect(page.get_by_role("heading", name="工程知识库", exact=True)).to_be_visible()
            expect(page.locator(".library-count")).to_contain_text(re.compile(r"\d+ 篇正文"))
            manifest = context.request.get("/release-manifest.json")
            assert manifest.ok and manifest.json()["release"] == release, "Stale production release"
            page.screenshot(path=str(output / "home.png"), full_page=True)

            page.get_by_role("button", name="搜索文章、专题或关键词", exact=True).click()
            query = page.locator('[data-md-component="search-query"]')
            results = page.locator('[data-md-component="search-result"]')
            query.fill("Outbox")
            outbox = results.locator('a[href*="S-DIST-06-outbox-inbox-fault-lab/"]').first
            expect(outbox).to_be_visible(timeout=20000)
            page.screenshot(path=str(output / "search-english.png"))
            outbox.click()
            page.wait_for_url("**" + OUTBOX + "**")
            expect(page.locator(".library-article h1")).to_contain_text("Outbox")
            page.get_by_role("button", name="搜索知识库", exact=True).click()
            query.fill("双分录")
            ledger = results.locator('a[href*="S-PAY-07-postgresql-double-entry-ledger-lab/"]').first
            expect(ledger).to_be_visible(timeout=20000)
            page.screenshot(path=str(output / "search-chinese.png"))
            ledger.click()
            page.wait_for_url("**" + ARTICLE + "**")
            expect(page.locator(".library-article h1")).to_contain_text("PostgreSQL 双分录账本")
            expect(page.locator(".library-article")).to_contain_text("并发扣款")
            canonical = page.locator('link[rel="canonical"]').get_attribute("href")
            assert canonical == base.rstrip("/") + ARTICLE, "Article canonical domain"
            page.locator('.md-sidebar--secondary a[href="#_2"]').click()
            expect(page).to_have_url(re.compile(r"#_2$"))
            expect(page.get_by_role("button", name="复制", exact=True).first).to_be_visible()
            page.screenshot(path=str(output / "article.png"))
            page.get_by_role("button", name="搜索知识库", exact=True).click()
            query.fill("zzzxqvnomatch987")
            expect(results).to_contain_text("没有找到符合条件的结果")
            query.press("Escape")
            expect(page.get_by_role("button", name="搜索知识库", exact=True)).to_be_focused()
            assert not errors, "Browser script errors: " + str(errors)
            (output / "result.json").write_text(json.dumps({
                "result": "passed", "release": release,
                "checks": ["home", "release", "article", "canonical", "toc", "code",
                           "english_search", "chinese_search", "empty_search", "focus", "page_errors"],
            }, indent=2) + "\n")
            print("Production browser checks passed: " + release)
        except Exception:
            page.screenshot(path=str(output / "failure.png"), full_page=True)
            raise
        finally:
            (output / "page-errors.json").write_text(json.dumps(errors, indent=2) + "\n")
            context.tracing.stop(path=str(output / "trace.zip"))
            browser.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="https://knowledgebase.wenyiapp.cn")
    parser.add_argument("--release", required=True)
    parser.add_argument("--output", type=Path, default=Path("smoke-results"))
    args = parser.parse_args()
    check(args.base_url, args.release, args.output)
