{
  _config+:: {
    dbNameFilter: 'datname!~"template.*"',
    gaussdbExporterSelector: '',
    groupLabels: if self.enableMultiCluster then ['job', 'cluster'] else ['job'],
    instanceLabels: ['instance', 'server'],
    enableMultiCluster: false,
  },
}
