// Compatibilidade next/image (Fase 4A): <img> nativo com as props usadas pelo app.
// Não há otimização/redimensionamento automático — divergência documentada no lote.
/* eslint-disable @next/next/no-img-element -- shim deliberado de <img> nativo */
export default function Image({ src, alt = "", width, height, fill, priority, unoptimized, placeholder, style, ...rest }) {
  const resolvedStyle = fill
    ? { position: "absolute", inset: 0, width: "100%", height: "100%", ...style }
    : { width, height, ...style };
  return (
    <img
      src={typeof src === "object" && src?.src ? src.src : src}
      alt={alt}
      decoding={priority ? "sync" : "async"}
      loading={priority ? "eager" : "lazy"}
      style={resolvedStyle}
      {...rest}
    />
  );
}
