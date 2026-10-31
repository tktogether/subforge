import { describe, it, expect } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import HelpPanel from './HelpPanel.vue'
import { vuetify } from '../plugins/vuetify'
import { pageHelp, type PageHelp } from '../content/help'

const global = {
  plugins: [vuetify],
  stubs: {
    RouterLink: RouterLinkStub,
    teleport: true,
    VDialog: { props: ['modelValue'], template: '<div v-if="modelValue"><slot /></div>' },
  },
}

describe('HelpPanel', () => {
  it('渲染根节点 help-panel；没传 label 时入口行就是该页 oneLiner', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    expect(w.find('[data-testid="help-toggle-label"]').text()).toContain(pageHelp('overview').oneLiner)
  })

  it('传了 label 时入口行用 label，不再重复 oneLiner（防与页面副标题重复）', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview'), label: '使用说明' }, global })
    expect(w.find('[data-testid="help-toggle-label"]').text()).toBe('使用说明')
    expect(w.text()).not.toContain(pageHelp('overview').oneLiner)
  })

  it('没有可展开内容时退化成静态一行（未知 helpKey 兜底不留空按钮）', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('unknown-page') }, global })
    expect(w.find('[data-testid="help-toggle"]').exists()).toBe(false)
    expect(w.find('[data-testid="help-oneliner"]').text()).toBe('该页面暂无说明')
  })

  it('默认折叠：仅显示入口行，详情不可见', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    expect(w.find('[data-testid="help-toggle"]').exists()).toBe(true)
    expect(w.find('[data-testid="help-body"]').exists()).toBe(false)
    expect(w.find('[data-testid="help-when"]').exists()).toBe(false)
    expect(w.find('[data-testid="help-steps"]').exists()).toBe(false)
  })

  it('展开后渲染 help-when', async () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    await w.find('[data-testid="help-toggle"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="help-when"]').exists()).toBe(true)
  })

  it('steps 非空渲染 help-steps', async () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    await w.find('[data-testid="help-toggle"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="help-steps"]').exists()).toBe(true)
  })

  it('concepts 非空渲染 help-concepts', async () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    await w.find('[data-testid="help-toggle"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="help-concepts"]').exists()).toBe(true)
  })

  it('faq 非空渲染 help-faq', async () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    await w.find('[data-testid="help-toggle"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="help-faq"]').exists()).toBe(true)
  })

  it('links 非空渲染 help-links 且含 router-link', async () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    await w.find('[data-testid="help-toggle"]').trigger('click')
    await flushPromises()
    const links = w.find('[data-testid="help-links"]')
    expect(links.exists()).toBe(true)
    expect(links.findAllComponents(RouterLinkStub).length).toBeGreaterThan(0)
  })

  it('dense=true 时不渲染 help-when（展开后仍隐藏）', async () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview'), dense: true }, global })
    await w.find('[data-testid="help-toggle"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="help-when"]').exists()).toBe(false)
  })

  it('所有可选段为空时仍渲染根节点', () => {
    const empty: PageHelp = { page: 'x', title: 'x', oneLiner: 'x', whenToUse: '' }
    const w = mount(HelpPanel, { props: { help: empty }, global })
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    expect(w.find('[data-testid="help-steps"]').exists()).toBe(false)
    expect(w.find('[data-testid="help-concepts"]').exists()).toBe(false)
    expect(w.find('[data-testid="help-faq"]').exists()).toBe(false)
    expect(w.find('[data-testid="help-links"]').exists()).toBe(false)
  })
})
