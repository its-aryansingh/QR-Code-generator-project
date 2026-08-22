import type { Metadata } from 'next';
import { QRTypePage } from '@/components/seo/QRTypePage';

export const metadata: Metadata = {
  title: 'Free WiFi QR Code Generator | QRit',
  description: 'Create a WiFi QR code instantly to let guests connect to your network without typing a password. Secure, fast, and free to use.',
  keywords: ['wifi qr code', 'wifi qr code generator', 'free wifi qr', 'qr code for wifi', 'connect wifi qr'],
  alternates: {
    canonical: 'https://qrit.com/wifi-qr-code'
  }
};

export default function WifiQRPage() {
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "WiFi QR Code Generator - QRit",
    "operatingSystem": "Web",
    "applicationCategory": "UtilitiesApplication",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "USD"
    }
  };

  const howToSteps = [
    { title: "Enter Network Details", desc: "Type in your exact WiFi Network Name (SSID) and Password." },
    { title: "Select Security Type", desc: "Choose your encryption type (WPA/WPA2, WEP, or None)." },
    { title: "Download & Share", desc: "Your QR code generates instantly. Download it to print or display." }
  ];

  const useCases = [
    { title: "Cafes & Restaurants", desc: "Let customers easily connect to your free guest WiFi by scanning a code on their table or the menu." },
    { title: "Hotels & Airbnbs", desc: "Provide a seamless check-in experience. Guests can connect to WiFi instantly without typing complex passwords." },
    { title: "Offices", desc: "Give visitors and clients secure access to your guest network during meetings." },
    { title: "Events", desc: "Make it simple for attendees at conferences and meetups to get online quickly." }
  ];

  const faqs = [
    { question: "Is it safe to share my WiFi via QR code?", answer: "Yes, it is just as safe as giving out your password manually. The QR code simply automates the process of entering the credentials." },
    { question: "Will this work with a hidden network?", answer: "Most modern smartphones support connecting to hidden networks via QR code, provided the SSID and password are correct, though some older devices may require manual connection." },
    { question: "What security type should I choose?", answer: "Most modern home and business networks use WPA or WPA2. If you are unsure, WPA is the safest default choice." },
    { question: "Do my guests need a special app to scan this?", answer: "No, the default camera app on both iOS and Android can read WiFi QR codes and prompt the user to connect." },
    { question: "Can I change the password later?", answer: "If you change your WiFi password, you will need to generate and print a new QR code, unless you are using a dynamic QR code feature (coming soon)." }
  ];

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <QRTypePage
        type="wifi"
        heroH1="Free WiFi QR Code Generator"
        heroSubtitle="Instantly create scannable QR codes for your WiFi network. Let your guests connect securely without typing a password."
        howToSteps={howToSteps}
        useCases={useCases}
        faqs={faqs}
      />
    </>
  );
}
