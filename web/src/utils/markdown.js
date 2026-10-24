import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/core'
import go from 'highlight.js/lib/languages/go'
import java from 'highlight.js/lib/languages/java'
import sql from 'highlight.js/lib/languages/sql'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import bash from 'highlight.js/lib/languages/bash'
import python from 'highlight.js/lib/languages/python'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'
import { Marked } from 'marked'
import 'highlight.js/styles/github.min.css'

hljs.registerLanguage('go', go)
hljs.registerLanguage('java', java)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('js', javascript)
hljs.registerLanguage('json', json)
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('sh', bash)
hljs.registerLanguage('python', python)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('yaml', yaml)

const marked = new Marked()

function escapeHtml(value) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
}

marked.use({
  gfm: true,
  breaks: true,
  renderer: {
    code(codeOrToken, infostring) {
      const text = typeof codeOrToken === 'string' ? codeOrToken : codeOrToken.text || ''
      const langRaw = typeof codeOrToken === 'string' ? infostring : codeOrToken.lang
      const lang = (langRaw || 'text').split(/\s+/)[0]
      let highlighted = escapeHtml(text)
      if (lang && hljs.getLanguage(lang)) {
        highlighted = hljs.highlight(text, { language: lang }).value
      }
      const label = lang && lang !== 'text' ? lang : '代码'
      return `<div class="codeblock"><div class="codehead"><span>${escapeHtml(label)}</span><button type="button" class="copy-code">复制</button></div><pre><code class="hljs language-${escapeHtml(lang)}">${highlighted}</code></pre></div>`
    },
  },
})

export function renderMarkdown(source) {
  const html = marked.parse(source || '', { async: false })
  return DOMPurify.sanitize(html, { ADD_ATTR: ['class'] })
}
