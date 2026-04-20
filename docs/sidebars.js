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
      items: ['sources/argocd', 'sources/kubeconform', 'sources/tenable-was'],
    },
    'templates',
  ],
};

module.exports = sidebars;
