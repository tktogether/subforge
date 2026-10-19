import { defineComponent, h } from 'vue'
import { useRouter } from 'vue-router'
import { config } from '@vue/test-utils'

function hrefOf(to: unknown): string {
  if (typeof to === 'string') return to
  if (to && typeof to === 'object') {
    const obj = to as { path?: string }
    return obj.path || '#'
  }
  return '#'
}

config.global.stubs = {
  ...config.global.stubs,
  RouterLink: defineComponent({
    name: 'RouterLinkStub',
    props: {
      to: { type: [String, Object], default: '' },
    },
    setup(props, { slots }) {
      let router: ReturnType<typeof useRouter> | null = null
      try {
        router = useRouter()
      } catch {
        router = null
      }
      return () =>
        h(
          'a',
          {
            href: hrefOf(props.to),
            onClick: (e: MouseEvent) => {
              e.preventDefault()
              if (router && props.to) {
                void router.push(props.to as any) // vue-router's push accepts string | RouteLocationRaw
              }
            },
          },
          slots.default?.(),
        )
    },
  }),
}
