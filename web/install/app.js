const form = document.getElementById('install-form')
const message = document.getElementById('install-message')

const setMessage = (text, type = '') => {
  message.textContent = text
  message.className = `install-message ${type}`.trim()
}

const checkStatus = async () => {
  try {
    const res = await fetch('/api/install/status', { cache: 'no-store' })
    if (!res.ok) throw new Error('无法检测安装状态')
    const data = await res.json()
    if (!data.needs_bootstrap) {
      setMessage('系统已完成初始化，可以直接访问登录页。', 'success')
      form.querySelectorAll('input, button').forEach((el) => (el.disabled = true))
    }
  } catch (err) {
    console.error(err)
    setMessage('检测安装状态失败，请稍后重试。', 'error')
  }
}

form.addEventListener('submit', async (evt) => {
  evt.preventDefault()
  setMessage('正在创建管理员账号...', '')
  const submitBtn = form.querySelector('button[type="submit"]')
  submitBtn.disabled = true

  const payload = {
    username: form.username.value.trim(),
    email: form.email.value.trim(),
    password: form.password.value.trim(),
  }

  if (!payload.email && !payload.username) {
    setMessage('请至少填写邮箱或用户名。', 'error')
    submitBtn.disabled = false
    return
  }

  try {
    const res = await fetch('/api/install', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      const detail = data?.error || '创建失败，请检查输入。'
      throw new Error(detail)
    }
    setMessage('管理员创建成功，正在跳转到登录页…', 'success')
    form.reset()
    setTimeout(() => {
      window.location.href = '/'
    }, 1200)
  } catch (err) {
    console.error(err)
    setMessage(err.message || '创建失败，请稍后再试。', 'error')
  } finally {
    submitBtn.disabled = false
  }
})

checkStatus()
