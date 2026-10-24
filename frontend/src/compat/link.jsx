// Compatibilidade next/link sobre react-router (Fase 4A).
// Aceita href/replace/prefetch e URLs externas; URLs absolutas viram <a> comum.
import { Link as RouterLink } from "react-router-dom";

const isExternal = (href) =>
  typeof href === "string" &&
  (/^[a-z][a-z0-9+.-]*:/i.test(href) || href.startsWith("//"));

export default function Link({ href, to, replace, prefetch, scroll, ...rest }) {
  const target = href ?? to;
  if (isExternal(target)) {
    return <a href={target} {...rest} />;
  }
  return <RouterLink to={target} replace={replace} {...rest} />;
}
