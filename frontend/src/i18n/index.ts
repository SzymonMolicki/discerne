import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import enUS from "./en-US.json";
import esES from "./es-ES.json";
import plPL from "./pl-PL.json";

export const supportedLocales = ["pl-PL", "en-US", "es-ES"] as const;

export function isSupportedLocale(value: string): value is (typeof supportedLocales)[number] {
  return supportedLocales.includes(value as (typeof supportedLocales)[number]);
}

i18n.use(initReactI18next).init({
  resources: {
    "pl-PL": { translation: plPL },
    "en-US": { translation: enUS },
    "es-ES": { translation: esES },
  },
  lng: resolveInitialLocale(),
  fallbackLng: "en-US",
  interpolation: {
    escapeValue: false,
  },
});

function resolveInitialLocale() {
  const savedLocale = window.localStorage.getItem("discerne_locale");
  if (savedLocale !== null && isSupportedLocale(savedLocale)) {
    return savedLocale;
  }

  for (const browserLocale of window.navigator.languages) {
    if (isSupportedLocale(browserLocale)) {
      return browserLocale;
    }
    if (browserLocale.startsWith("pl")) {
      return "pl-PL";
    }
    if (browserLocale.startsWith("es")) {
      return "es-ES";
    }
  }

  return "en-US";
}
