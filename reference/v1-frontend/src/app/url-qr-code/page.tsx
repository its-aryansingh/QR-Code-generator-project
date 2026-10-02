import type { Metadata } from 'next';
import { QRTypePage } from '@/components/seo/QRTypePage';

export const metadata: Metadata = {
  title: 'Free URL & Website QR Code Generator | QRit',
  description: 'Convert any web address into a QR code for free. Perfect for marketing materials, packaging, and sharing links instantly.',
  keywords: ['url qr code', 'website qr code', 'link to qr code', 'qr code generator url'],
  alternates: {
    canonical: 'https://qrit.com/url-qr-code'
  }
};

export default function UrlQRPage() {
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "URL QR Code Generator - QRit",
    "operatingSystem": "Web",
    "applicationCategory": "UtilitiesApplication",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "USD"
    }
  };

  const howToSteps = [
    { title: "Copy Your Link", desc: "Copy the full URL (including https://) of the website you want to share." },
    { title: "Paste URL", desc: "Paste your link into the generator input field." },
    { title: "Generate & Download", desc: "Your QR code is ready instantly. Download and start sharing." }
  ];

  const useCases = [
    { title: "Marketing Campaigns", desc: "Drive offline traffic to your online landing pages by placing QR codes on posters and flyers." },
    { title: "Product Packaging", desc: "Link customers to user manuals, warranty registrations, or instructional videos directly from the box." },
    { title: "Restaurant Menus", desc: "Replace physical menus with a QR code that links to a digital PDF or ordering system." },
    { title: "Event Promotions", desc: "Direct attendees to ticketing pages, schedules, or event maps." }
  ];

  const faqs = [
    { question: "Do URL QR codes expire?", answer: "No, static URL QR codes never expire as long as the website link itself remains active." },
    { question: "What if my link is very long?", answer: "Long links make the QR code pattern more dense and harder to scan. It's recommended to use a URL shortener or a Dynamic QR code for very long URLs." },
    { question: "Can I track how many times it was scanned?", answer: "Static QR codes cannot be tracked. To get scan analytics, you will need to sign up and create a Dynamic QR code." },
    { question: "Do I need to include 'https://'?", answer: "Yes, including 'https://' ensures that the scanning device recognizes it as a clickable web link." },
    { question: "Why is my QR code not scanning?", answer: "Ensure there is enough contrast between the code and the background, the QR code is large enough, and the URL is correct." }
  ];

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <QRTypePage
        type="url"
        heroH1="Free URL QR Code Generator"
        heroSubtitle="Turn any website link into a scannable QR code instantly. Drive offline traffic to your online content effortlessly."
        howToSteps={howToSteps}
        useCases={useCases}
        faqs={faqs}
      />
    </>
  );
}
