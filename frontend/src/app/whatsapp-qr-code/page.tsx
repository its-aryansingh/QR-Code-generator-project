import type { Metadata } from 'next';
import { QRTypePage } from '@/components/seo/QRTypePage';

export const metadata: Metadata = {
  title: 'Free WhatsApp QR Code Generator | QRit',
  description: 'Create a WhatsApp QR code to let customers message you instantly. Pre-fill messages and share easily. Free and no signup required.',
  keywords: ['whatsapp qr code', 'whatsapp link generator', 'qr code for whatsapp', 'wa.me qr code'],
  alternates: {
    canonical: 'https://qrit.com/whatsapp-qr-code'
  }
};

export default function WhatsappQRPage() {
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "WhatsApp QR Code Generator - QRit",
    "operatingSystem": "Web",
    "applicationCategory": "CommunicationApplication",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "USD"
    }
  };

  const howToSteps = [
    { title: "Enter Phone Number", desc: "Select your country code and enter your WhatsApp phone number." },
    { title: "Add Pre-filled Message", desc: "Optional: type a message that will automatically appear when they scan the code." },
    { title: "Download", desc: "Download the generated QR code and place it on your storefront or marketing materials." }
  ];

  const useCases = [
    { title: "Customer Support", desc: "Allow customers to reach your support team instantly without saving your phone number." },
    { title: "Small Businesses", desc: "Take orders for food, services, or products directly via WhatsApp." },
    { title: "Real Estate & Sales", desc: "Let interested buyers inquire about a specific property by scanning a code on a signboard." },
    { title: "Print Advertising", desc: "Convert offline readers into engaged leads by letting them message you instantly from a flyer." }
  ];

  const faqs = [
    { question: "Do customers need to save my number first?", answer: "No! That's the main benefit. Scanning the QR code opens a chat with you directly in WhatsApp without them needing to save your contact." },
    { question: "What is a pre-filled message?", answer: "It's text that is automatically typed into the user's message box when they scan the code. They just have to hit 'Send'." },
    { question: "Does this work with WhatsApp Business?", answer: "Yes, it works perfectly with both standard WhatsApp and WhatsApp Business accounts." },
    { question: "Are WhatsApp QR codes free?", answer: "Yes, generating a WhatsApp QR code with QRit is completely free and static codes never expire." },
    { question: "Can I use this for a WhatsApp Group?", answer: "This specific generator is for direct messaging a number. For groups, use the 'URL QR Code' generator with your group invite link." }
  ];

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <QRTypePage
        type="whatsapp"
        heroH1="Free WhatsApp QR Code Generator"
        heroSubtitle="Make it effortless for customers to contact you on WhatsApp. Generate a QR code with a pre-filled message instantly."
        howToSteps={howToSteps}
        useCases={useCases}
        faqs={faqs}
      />
    </>
  );
}
