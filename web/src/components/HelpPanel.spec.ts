import { describe, it, expect } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import HelpPanel from './HelpPanel.vue'
import { vuetify } from '../plugins/vuetify'
import { pageHelp, type PageHelp } from '../content/help'

const global = { plugins: [vuetify], stubs: { RouterLink: RouterLinkStub } }

describe('HelpPanel', () => {
  it('渲染根节点 help-panel + help-oneliner', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    expect(w.find('[data-testid="help-panel"]').exists()).toBe(true)
    expect(w.find('[data-testid="help-oneliner"]').text()).toContain(pageHelp('overview').oneLiner)
  })

  it('whenToUse 非空时渲染 help-when', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    expect(w.find('[data-testid="help-when"]').exists()).toBe(true)
  })

  it('steps 非空渲染 help-steps', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    expect(w.find('[data-testid="help-steps"]').exists()).toBe(true)
  })

  it('concepts 非空渲染 help-concepts', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    expect(w.find('[data-testid="help-concepts"]').exists()).toBe(true)
  })

  it('faq 非空渲染 help-faq', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    expect(w.find('[data-testid="help-faq"]').exists()).toBe(true)
  })

  it('links 非空渲染 help-links 且含 router-link', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview') }, global })
    const links = w.find('[data-testid="help-links"]')
    expect(links.exists()).toBe(true)
    expect(links.findAllComponents(RouterLinkStub).length).toBeGreaterThan(0)
  })

  it('dense=true 时不渲染 help-when', () => {
    const w = mount(HelpPanel, { props: { help: pageHelp('overview'), dense: true }, global })
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
