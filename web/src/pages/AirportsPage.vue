<script setup lang="ts">
import { endpoints } from '../api/endpoints'
import ResourcePage from '../components/ResourcePage.vue'

const resource: any = {
  list: endpoints.listAirports,
  create: endpoints.createAirport,
  update: (item: any) => endpoints.updateAirport(item.id, item),
  remove: (item: any) => endpoints.deleteAirport(item.id),
}
</script>

<template>
  <ResourcePage
    title="机场"
    subtitle="订阅源机场"
    :columns="[{ key: 'name', title: '名称' }, { key: 'subscription_url', title: '订阅地址' }, { key: 'remarks', title: '备注' }, { key: 'enabled', title: '启用' }]"
    :fields="[{ key: 'name', label: '名称', type: 'text' }, { key: 'subscription_url', label: '订阅地址', type: 'text' }, { key: 'remarks', label: '备注', type: 'text' }, { key: 'enabled', label: '启用', type: 'switch' }]"
    :resource="resource"
  />
</template>