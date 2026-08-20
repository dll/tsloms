"""将 TSLOMS 中文 Markdown 文档转换为 PDF；供发布前文档构建使用。"""
from pathlib import Path
import sys
import markdown

src = Path(sys.argv[1]).resolve()
dst = Path(sys.argv[2]).resolve()
body = markdown.markdown(src.read_text(encoding="utf-8"), extensions=["tables", "fenced_code", "toc"])
html = f'''<!doctype html><html><head><meta charset="utf-8"><title>{src.stem}</title></head><body><h1>TSLOMS 信号灯检测器接入操作</h1>{body}</body></html>'''
css = '''<style>@page { size: A4; margin: 18mm 16mm; } body { font-family: "Microsoft YaHei", "Noto Sans CJK SC", sans-serif; font-size: 10.5pt; line-height: 1.65; color: #1f2937; } h1 { color: #0f4c81; border-bottom: 2px solid #0f4c81; padding-bottom: 6px; } h2 { color: #166534; margin-top: 18px; } table { width: 100%; border-collapse: collapse; margin: 8px 0 14px; } th, td { border: 1px solid #cbd5e1; padding: 5px 7px; vertical-align: top; } th { background: #e2e8f0; } code, pre { font-family: Consolas, monospace; background: #f1f5f9; } pre { padding: 8px; border-left: 3px solid #0f4c81; white-space: pre-wrap; } blockquote { border-left: 4px solid #94a3b8; padding-left: 10px; color: #475569; }</style>'''
html = html.replace('</head>', css + '</head>')
tmp = dst.with_suffix('.html')
tmp.write_text(html, encoding='utf-8')
print(tmp)
