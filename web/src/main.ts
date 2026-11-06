// 字体本地打包：Tektur（展示）/ Archivo（正文）/ Martian Mono（数据）
import '@fontsource-variable/tektur'
import '@fontsource-variable/archivo'
import '@fontsource-variable/martian-mono'
import { createSubforgeApp } from './bootstrap'
import './design/tokens.css'
import './styles/index.css'

createSubforgeApp().mount('#app')
