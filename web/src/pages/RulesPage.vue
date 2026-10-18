<script setup lang="ts">
import { endpoints } from '../api/endpoints'
import ResourcePage from '../components/ResourcePage.vue'

const resource: any = {
  list: endpoints.listRules,
  create: endpoints.createRule,
  update: (item: any) => endpoints.updateRule(item.id, item),
  remove: (item: any) => endpoints.deleteRule(item.id),
}
</script>

<template>
  <ResourcePage
    title="规则"
    subtitle="转换规则"
    :columns="[{ key: 'rule_type', title: '类型' }, { key: 'rule_param', title: '参数' }, { key: 'rule_config', title: '配置' }, { key: 'resolve_dns', title: '解析DNS' }, { key: 'enabled', title: '启用' }]"
    :fields="[{ key: 'rule_type', label: '类型', type: 'text' }, { key: 'rule_param', label: '参数', type: 'text' }, { key: 'rule_config', label: '配置', type: 'number' }, { key: 'resolve_dns', label: '解析DNS', type: 'switch' }, { key: 'enabled', label: '启用', type: 'switch' }]"
    :resource="resource"
  />
</template>