// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docs: [
    'intro',
    'installation',
    {
      type: 'category',
      label: 'Sources',
      collapsed: false,
      items: ['sources/argocd', 'sources/kubeconform', 'sources/tenable-was', 'sources/cyclonedx', 'sources/dependency-check', 'sources/sonarqube', 'sources/trivy'],
    },
    'templates',
  ],
};

module.exports = sidebars;
