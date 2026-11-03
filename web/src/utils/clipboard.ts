// 复制文本：优先用异步 Clipboard API，HTTP（非安全上下文）下回退到
// 临时 textarea + execCommand。浏览器在 http://<局域网 IP> 下通常不提供
// navigator.clipboard，或提供但拒绝写入；只依赖它会让复制按钮直接失效。
export async function copyText(text: string): Promise<boolean> {
  const clipboard = navigator.clipboard
  if (clipboard && typeof clipboard.writeText === 'function') {
    try {
      await clipboard.writeText(text)
      return true
    } catch {
      // 非安全上下文/权限拒绝：继续走兼容路径。
    }
  }
  return fallbackCopy(text)
}

function fallbackCopy(text: string): boolean {
  const area = document.createElement('textarea')
  area.value = text
  area.setAttribute('readonly', '')
  area.style.position = 'fixed'
  area.style.top = '0'
  area.style.left = '-9999px'
  area.style.opacity = '0'
  document.body.appendChild(area)
  try {
    area.focus()
    area.select()
    area.setSelectionRange(0, area.value.length)
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    document.body.removeChild(area)
  }
}
