import { registerDeprecationHandler } from '@ember/debug';

// https://guides.emberjs.com/release/configuring-ember/handling-deprecations/#toc_filtering-deprecations
export function initialize() {
  registerDeprecationHandler((message, options, next) => {
    // filter deprecations that are scheduled to be removed in a specific version
    // when upgrading or addressing deprecation warnings be sure to update this or remove if not needed
    if (options?.until !== '4.0.0') {
      next(message, options);
    }
    return;
  });
}

export default { initialize };
